// 本文件把登记的上游 SSE 解码为文本/工具/用量，再编码到客户端协议；必须看到可结束终态才可提交完成。
// Responses 的 incomplete 允许结束，但保留生成未完整完成的原因；失败或缺失终态不提交完成。
// 调用关系与边界：统一对话请求构造与流协议转换；具体参数、返回及副作用见各方法说明。
package dialogue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/openai/openai-go/v3/packages/ssestream"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// dialogueStream 是单次协议流转换的状态机，由 PipeDialogue 创建并驱动。
// 累积统一文本、工具调用和用量，再编码为客户端所需 SSE；上游索引、输出内容块和 Chat 工具索引分别管理。
// started 表示开始事件已处理，wrote 表示实际写出过字节，completed 表示看到了可结束终态，三者不能互相替代。
// Responses 的 incomplete 也可结束，但会保留 length 原因，不伪装成完整生成。
// 只在当前请求内串行使用；完成前验证工具参数，Responses 终态前提交已验证的续接历史。
type dialogueStream struct {
	// 客户端 HTTP 写入器，写错误沿转换链返回。
	writer http.ResponseWriter
	// 转换后 SSE 的日志捕获缓冲，写客户端前记录，因此可能包含未实际送达的尾片段。
	capture bytes.Buffer
	// 客户端协议、公开模型、响应 ID、累计文本与归一结束原因；均为本请求私有状态。
	protocol, model, id, text, stop string
	// 上游工具索引到累积调用的映射，逐帧拼接名称和 JSON 参数。
	calls map[int]*llmdialogue.Turn
	// 上游工具索引到稳定输出内容块索引的映射，避免与文本块碰撞。
	blocks map[int]int
	// 本次上游报告的统计，分帧覆盖字段，不按每帧重复求和。
	usage map[string]any
	// 开始事件已处理、实际写出过字节、看到可结束终态三个独立事实；wrote 后不可重试。
	started, wrote, completed bool
	// 下一个可分配的输出块索引，文本和工具共用。
	nextBlock int
	// 文本块的稳定索引，-1 表示尚未创建。
	textBlock int
	// Responses 事件单调递增序号，emit 给事件对象补入。
	sequence int
	// 稀疏上游工具索引到 Chat 连续工具索引的映射。
	toolIndices map[int]int
	// 可选完成回调，参数为已验证 DialogueResult，无返回；Responses 终态发出前提交历史。
	// 终态校验前失败不调用；回调之后若客户端写入失败，已提交历史不回滚。
	beforeComplete func(llmdialogue.DialogueResult)
}

// streamMap 读取流事件中的动态对象字段。
// 参数 v 为任意 JSON 值；返回 map 或 nil；consume 等解码辅助方法调用。
// 非对象不报错，后续协议分支负责语义校验；不复制对象、不修改输入。
func streamMap(v any) map[string]any { m, _ := v.(map[string]any); return m }

// streamString 读取流事件中的动态字符串字段。
// 参数 v 为任意值；返回字符串，类型不匹配或 nil 返回空串；流事件解码调用。
// 不将数字或对象隐式转成字符串，不产生外部副作用；必需字段的缺失由完成校验处理。
func streamString(v any) string { s, _ := v.(string); return s }

// streamIndex 读取普通 JSON 解码得到的流事件索引。
// 参数 v 应为 float64；返回截断整数，其他类型返回零；consume 用于定位工具或输出项。
// 不做索引范围/唯一性验证，也不支持所有数字类型；只是当前解码方式的局部辅助函数。
func streamIndex(v any) int { n, _ := v.(float64); return int(n) }

// emit 将一条调用方协议事件编码成完整 SSE 帧，并立即尝试刷新客户端。
// 参数 event 为可空事件名，data 为 JSON 数据；返回编码/写入错误，由全部输出方法调用。
// Responses 为对象原地加入递增 sequence_number；先捕获拟发送帧，再 Write，实际 n>0 才置 wrote。
// capture 可能包含未完全写成功的帧，因此不能作为客户端完整接收的证明；写出后执行器必须禁止换部署。
// 副作用为状态序号、捕获缓冲和 HTTP 输出；刷新只在 writer 支持 http.Flusher 时执行。
func (s *dialogueStream) emit(event string, data any) error {
	if s.protocol == "openai-responses" {
		if m, ok := data.(map[string]any); ok {
			m["sequence_number"] = s.sequence
			s.sequence++
		}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var b strings.Builder
	if event != "" {
		fmt.Fprintf(&b, "event: %s\n", event)
	}
	fmt.Fprintf(&b, "data: %s\n\n", raw)
	chunk := []byte(b.String())
	s.capture.Write(chunk)
	n, err := s.writer.Write(chunk)
	if n > 0 {
		s.wrote = true
	}
	if f, ok := s.writer.(http.Flusher); ok {
		f.Flush()
	}
	return err
}

// start 按调用方协议发送首次生命周期事件，保证同一请求只开始一次。
// 无参数；返回写入/不支持协议错误；textDelta、toolDelta 与完成处理在首次输出前调用。
// Chat 输出 assistant 角色，Responses 输出 created，Messages 输出 message_start；Google 不需要独立开始帧。
// 修改 started 并可能写 HTTP；重复调用直接成功，不重置 ID、用量或块索引。
func (s *dialogueStream) start() error {
	if s.started {
		return nil
	}
	s.started = true
	switch s.protocol {
	case "gemini", "vertex":
		return nil
	case "openai-chat":
		return s.chat(map[string]any{"role": "assistant"}, nil)
	case "openai-responses":
		return s.emit("response.created", map[string]any{"type": "response.created", "response": map[string]any{"id": s.id, "object": "response", "model": s.model, "status": "in_progress", "store": false, "output": []any{}}})
	case "anthropic-messages":
		return s.emit("message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": s.id, "type": "message", "role": "assistant", "model": s.model, "content": []any{}, "stop_reason": nil, "usage": map[string]any{"input_tokens": s.usage["prompt_tokens"], "output_tokens": 0}}})
	}
	return fmt.Errorf("unsupported stream protocol")
}

// chat 构造调用方 Chat 协议的单个增量 chunk。
// 参数 delta 为角色/文本/工具增量，finish 为可空结束原因；返回 emit 的写入错误。
// Chat 输出分支调用；使用本轮响应 ID、公开模型和首个 choice，当前时间作为 created。
// 只负责封装，不累计 token 或校验终态；通过 emit 改变捕获与写出状态。
func (s *dialogueStream) chat(delta map[string]any, finish any) error {
	return s.emit("", map[string]any{"id": s.id, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": s.model, "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
}

// textDelta 累积并按调用方协议输出一段文本增量。
// 参数 text 为新增文本；返回写入错误，空文本直接成功；consume 解码文本事件后调用。
// 确保开始事件已发出，将文本累积到 s.text；Responses/Messages 首次文本创建固定内容块，再发送增量。
// Google/Chat 直接输出相应文本 chunk；输出块与工具共用分配器以保持最终 output_index 对应。
// 会修改请求状态并写 HTTP，不把空文本当作有效生成或完成证明。
func (s *dialogueStream) textDelta(text string) error {
	if text == "" {
		return nil
	}
	if err := s.start(); err != nil {
		return err
	}
	s.text += text
	switch s.protocol {
	case "gemini", "vertex":
		return s.emit("", map[string]any{"candidates": []any{map[string]any{"index": 0, "content": map[string]any{"role": "model", "parts": []any{map[string]any{"text": text}}}}}})
	case "openai-chat":
		return s.chat(map[string]any{"content": text}, nil)
	case "openai-responses":
		if s.textBlock < 0 {
			s.textBlock = s.nextBlock
			s.nextBlock++
			if err := s.emit("response.output_item.added", map[string]any{"type": "response.output_item.added", "output_index": s.textBlock, "item": map[string]any{"id": s.id + "_message", "type": "message", "role": "assistant", "status": "in_progress", "content": []any{}}}); err != nil {
				return err
			}
			if err := s.emit("response.content_part.added", map[string]any{"type": "response.content_part.added", "item_id": s.id + "_message", "output_index": s.textBlock, "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}}); err != nil {
				return err
			}
		}
		return s.emit("response.output_text.delta", map[string]any{"type": "response.output_text.delta", "item_id": s.id + "_message", "output_index": s.textBlock, "content_index": 0, "delta": text})
	case "anthropic-messages":
		if s.textBlock < 0 {
			s.textBlock = s.nextBlock
			s.nextBlock++
			if err := s.emit("content_block_start", map[string]any{"type": "content_block_start", "index": s.textBlock, "content_block": map[string]any{"type": "text", "text": ""}}); err != nil {
				return err
			}
		}
		return s.emit("content_block_delta", map[string]any{"type": "content_block_delta", "index": s.textBlock, "delta": map[string]any{"type": "text_delta", "text": text}})
	}
	return nil
}

// toolDelta 将分散的工具 ID、函数名和 JSON 参数片段归并到同一工具调用。
// 参数 index 为上游工具索引，id、name、args 为本帧增量；返回协议输出错误，consume/Google 工具转换调用。
// 首次出现时分配稳定输出块，同时为 Chat 分配从零连续的工具索引；上游稀疏索引不能直接泄露为 Chat 索引。
// ID 非空时更新，名称和参数按增量追加；参数完整性、必需 ID/名称在 finish 统一校验。
// 会修改 calls/blocks/toolIndices 并写流，不能跨请求复用或在终态失败后提交历史。
func (s *dialogueStream) toolDelta(index int, id, name, args string) error {
	if err := s.start(); err != nil {
		return err
	}
	call := s.calls[index]
	fresh := call == nil
	if fresh {
		call = &llmdialogue.Turn{}
		s.calls[index] = call
		s.toolIndices[index] = len(s.toolIndices)
		s.blocks[index] = s.nextBlock
		s.nextBlock++
	}
	if id != "" {
		call.ID = id
	}
	call.Name += name
	call.Arguments += args
	block := s.blocks[index]
	switch s.protocol {
	case "openai-chat":
		fn := map[string]any{}
		if name != "" {
			fn["name"] = name
		}
		if args != "" {
			fn["arguments"] = args
		}
		c := map[string]any{"index": s.toolIndices[index], "function": fn}
		if fresh || id != "" {
			c["id"] = call.ID
			c["type"] = "function"
		}
		return s.chat(map[string]any{"tool_calls": []any{c}}, nil)
	case "openai-responses":
		if fresh {
			if err := s.emit("response.output_item.added", map[string]any{"type": "response.output_item.added", "output_index": block, "item": map[string]any{"id": call.ID, "type": "function_call", "call_id": call.ID, "name": call.Name, "arguments": "", "status": "in_progress"}}); err != nil {
				return err
			}
		}
		if args != "" {
			return s.emit("response.function_call_arguments.delta", map[string]any{"type": "response.function_call_arguments.delta", "output_index": block, "item_id": call.ID, "delta": args})
		}
	case "anthropic-messages":
		if fresh {
			if err := s.emit("content_block_start", map[string]any{"type": "content_block_start", "index": block, "content_block": map[string]any{"type": "tool_use", "id": call.ID, "name": call.Name, "input": map[string]any{}}}); err != nil {
				return err
			}
		}
		if args != "" {
			return s.emit("content_block_delta", map[string]any{"type": "content_block_delta", "index": block, "delta": map[string]any{"type": "input_json_delta", "partial_json": args}})
		}
	}
	return nil
}

// observeUsage 保存上游流事件报告的用量，并归一输入/输出字段别名。
// 参数 u 为部分用量，可为空；返回值无，consume 的用量事件分支调用。
// 后到同名字段覆盖旧值而非累加，保留后续帧省略的字段；本方法没有递归合并嵌套明细。
// 只更新本请求 s.usage，不估算、不记账，缺少终态时这些已观测事实也不能使失败流成为成功。
func (s *dialogueStream) observeUsage(u map[string]any) {
	for k, v := range u {
		switch k {
		case "input_tokens":
			s.usage["prompt_tokens"] = v
		case "output_tokens":
			s.usage["completion_tokens"] = v
		default:
			s.usage[k] = v
		}
	}
}

// finish 验证累计回复并输出调用方协议要求的完成事件。
// 无参数；返回工具不完整、参数 JSON 非法或写出错误；PipeDialogue 确认上游完成后调用。
// 按稳定索引收集工具，校验 ID、名称与完整 JSON 参数，再归一用量和 stop 原因。
// 各协议使用自己的结束块/终态；Responses 最终 output 保留增量阶段索引，在 completed/incomplete 发布前调用 beforeComplete。
// 回调收到验证过的 DialogueResult，先保存续接归属再发布终态，避免客户端立即续接时找不到历史。
// 若随后终态写入失败，已保存归属不会回滚；回调不是客户端接收确认，也不能保存未验证的工具参数。
// 副作用为 HTTP 帧与回调状态写入，函数不能修复已发生的部分写出。
func (s *dialogueStream) finish() error {
	if err := s.start(); err != nil {
		return err
	}
	s.usage = llmdialogue.NormalizeDialogueUsage(s.usage)
	r := llmdialogue.DialogueResult{ID: s.id, Text: s.text, Stop: s.stop, Usage: s.usage}
	if r.Stop == "" {
		r.Stop = "stop"
	}
	indices := make([]int, 0, len(s.calls))
	for i := range s.calls {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	for _, i := range indices {
		if c := s.calls[i]; c != nil {
			// 流可以在任意参数片段后中断，必须等终态时验证完整 JSON，不能把半截调用交给客户端执行。
			if c.Arguments == "" {
				c.Arguments = "{}"
			}
			if c.ID == "" || c.Name == "" || !json.Valid([]byte(c.Arguments)) {
				return fmt.Errorf("incomplete upstream tool call")
			}
			r.Calls = append(r.Calls, *c)
		}
	}
	if len(r.Calls) > 0 {
		r.Stop = "tool_calls"
	}
	switch s.protocol {
	case "gemini", "vertex":
		// 文本已按增量发送；终态仅发送完整函数参数与用量，避免文本重复。
		r.Text = ""
		raw, err := llmdialogue.EncodeDialogueResult(r, s.protocol, s.model)
		if err != nil {
			return err
		}
		var out map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			return err
		}
		return s.emit("", out)
	case "openai-chat":
		if err := s.chat(map[string]any{}, r.Stop); err != nil {
			return err
		}
		if err := s.emit("", map[string]any{"id": s.id, "object": "chat.completion.chunk", "model": s.model, "choices": []any{}, "usage": s.usage}); err != nil {
			return err
		}
		s.capture.WriteString("data: [DONE]\n\n")
		_, err := io.WriteString(s.writer, "data: [DONE]\n\n")
		if f, ok := s.writer.(http.Flusher); ok {
			f.Flush()
		}
		return err
	case "openai-responses":
		raw, err := llmdialogue.EncodeDialogueResult(r, s.protocol, s.model)
		if err != nil {
			return err
		}
		var response map[string]any
		_ = json.Unmarshal(raw, &response)
		// 完成事件按已发送的输出索引重排，保留工具和文本块的生命周期及稳定 ID。
		items := make([]any, s.nextBlock)
		if s.textBlock >= 0 {
			part := map[string]any{"type": "output_text", "text": s.text, "annotations": []any{}}
			item := map[string]any{"id": s.id + "_message", "type": "message", "role": "assistant", "status": "completed", "content": []any{part}}
			items[s.textBlock] = item
			for _, event := range []map[string]any{
				{"type": "response.output_text.done", "item_id": s.id + "_message", "output_index": s.textBlock, "content_index": 0, "text": s.text},
				{"type": "response.content_part.done", "item_id": s.id + "_message", "output_index": s.textBlock, "content_index": 0, "part": part},
				{"type": "response.output_item.done", "output_index": s.textBlock, "item": item},
			} {
				if err := s.emit(streamString(event["type"]), event); err != nil {
					return err
				}
			}
		}
		for _, index := range indices {
			c := s.calls[index]
			block := s.blocks[index]
			item := map[string]any{"id": c.ID, "type": "function_call", "call_id": c.ID, "name": c.Name, "arguments": c.Arguments, "status": "completed"}
			items[block] = item
			if err := s.emit("response.function_call_arguments.done", map[string]any{"type": "response.function_call_arguments.done", "item_id": c.ID, "output_index": block, "arguments": c.Arguments}); err != nil {
				return err
			}
			if err := s.emit("response.output_item.done", map[string]any{"type": "response.output_item.done", "output_index": block, "item": item}); err != nil {
				return err
			}
		}
		response["output"] = items
		event := "response.completed"
		if r.Stop == "length" {
			event = "response.incomplete"
		}
		if s.beforeComplete != nil {
			// 完成事件一旦可见客户端就可能立刻续接，因此先保存历史；这不是跨 HTTP/缓存的原子事务。
			s.beforeComplete(r)
		}
		return s.emit(event, map[string]any{"type": event, "response": response})
	case "anthropic-messages":
		for i := 0; i < s.nextBlock; i++ {
			if err := s.emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": i}); err != nil {
				return err
			}
		}
		stop := "end_turn"
		if r.Stop == "length" {
			stop = "max_tokens"
		}
		if len(r.Calls) > 0 {
			stop = "tool_use"
		}
		if err := s.emit("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stop, "stop_sequence": nil}, "usage": map[string]any{"output_tokens": s.usage["completion_tokens"]}}); err != nil {
			return err
		}
		return s.emit("message_stop", map[string]any{"type": "message_stop"})
	}
	return nil
}

// consume 按明确的上游协议解码单个完整 SSE data 事件。
// 参数 protocol 为登记协议，data 为事件文本；返回解析、供应商错误或调用方写出错误；PipeDialogue 逐事件调用。
// 解析 Chat、Responses、Messages、Google 的文本、工具、用量及终态，统一交给本请求输出状态机。
// 不从 URL 猜协议；DONE/协议结束标记才更新完成事实，网络 EOF 本身不能证明成功。
// 会更新响应 ID、stop、completed 和累计内容，并可能立即写出；工具/output item ID 不应覆盖响应级 ID。
func (s *dialogueStream) consume(protocol string, data string) error {
	if data == "[DONE]" {
		if protocol == "openai-chat" {
			s.completed = true
		}
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return err
	}
	if m["error"] != nil {
		return fmt.Errorf("upstream stream error")
	}
	if id := streamString(m["id"]); id != "" {
		s.id = id
	}
	s.observeUsage(streamMap(m["usage"]))
	typ := streamString(m["type"])
	switch protocol {
	case "gemini", "vertex":
		if m["candidates"] == nil {
			// Google 可在单独末帧提供用量；不能把缺少候选的普通空帧视为完成。
			if streamMap(m["usageMetadata"]) != nil {
				m["candidates"] = []any{map[string]any{}}
				raw, _ := json.Marshal(m)
				result, err := llmdialogue.ParseDialogueResult(protocol, raw)
				if err != nil {
					return err
				}
				for k, v := range result.Usage {
					if k != "pricing_blocked" {
						s.usage[k] = v
					}
				}
				return nil
			}
			return fmt.Errorf("Google stream candidate missing")
		}
		r, err := llmdialogue.ParseDialogueResult(protocol, []byte(data))
		if err != nil {
			return err
		}
		if r.ID != "" {
			s.id = r.ID
		}
		for k, v := range r.Usage {
			if k != "pricing_blocked" {
				s.usage[k] = v
			}
		}
		if err := s.textDelta(r.Text); err != nil {
			return err
		}
		for _, c := range r.Calls {
			if err := s.googleCall(c); err != nil {
				return err
			}
		}
		choices, _ := m["candidates"].([]any)
		if len(choices) == 1 && streamString(streamMap(choices[0])["finishReason"]) != "" {
			s.completed = true
			s.stop = r.Stop
		}
	case "openai-chat":
		choices, _ := m["choices"].([]any)
		for _, v := range choices {
			choice := streamMap(v)
			if streamIndex(choice["index"]) != 0 {
				return fmt.Errorf("multiple stream choices unsupported")
			}
			delta := streamMap(choice["delta"])
			if err := s.textDelta(streamString(delta["content"])); err != nil {
				return err
			}
			calls, _ := delta["tool_calls"].([]any)
			for _, v := range calls {
				c := streamMap(v)
				f := streamMap(c["function"])
				if err := s.toolDelta(streamIndex(c["index"]), streamString(c["id"]), streamString(f["name"]), streamString(f["arguments"])); err != nil {
					return err
				}
			}
			if stop := streamString(choice["finish_reason"]); stop != "" {
				s.stop = stop
			}
		}
	case "openai-responses":
		if response := streamMap(m["response"]); response != nil {
			s.id = streamString(response["id"])
			s.observeUsage(streamMap(response["usage"]))
		}
		switch typ {
		case "response.output_text.delta":
			return s.textDelta(streamString(m["delta"]))
		case "response.output_item.added":
			item := streamMap(m["item"])
			if item["type"] == "function_call" {
				return s.toolDelta(streamIndex(m["output_index"]), streamString(item["call_id"]), streamString(item["name"]), streamString(item["arguments"]))
			}
		case "response.function_call_arguments.delta":
			return s.toolDelta(streamIndex(m["output_index"]), "", "", streamString(m["delta"]))
		case "response.completed":
			s.completed = true
		case "response.incomplete":
			s.completed = true
			s.stop = "length"
		case "response.failed", "error":
			return fmt.Errorf("upstream response failed")
		}
	case "anthropic-messages":
		switch typ {
		case "message_start":
			msg := streamMap(m["message"])
			s.id = streamString(msg["id"])
			s.observeUsage(streamMap(msg["usage"]))
		case "content_block_start":
			b := streamMap(m["content_block"])
			switch b["type"] {
			case "text":
				return s.textDelta(streamString(b["text"]))
			case "tool_use":
				args := ""
				if input := streamMap(b["input"]); len(input) > 0 {
					raw, _ := json.Marshal(input)
					args = string(raw)
				}
				return s.toolDelta(streamIndex(m["index"]), streamString(b["id"]), streamString(b["name"]), args)
			default:
				return fmt.Errorf("unsupported upstream content block %v", b["type"])
			}
		case "content_block_delta":
			d := streamMap(m["delta"])
			switch d["type"] {
			case "text_delta":
				return s.textDelta(streamString(d["text"]))
			case "input_json_delta":
				return s.toolDelta(streamIndex(m["index"]), "", "", streamString(d["partial_json"]))
			default:
				return fmt.Errorf("unsupported upstream delta %v", d["type"])
			}
		case "message_delta":
			d := streamMap(m["delta"])
			if d["stop_reason"] == "max_tokens" {
				s.stop = "length"
			}
		case "message_stop":
			s.completed = true
		case "error":
			return fmt.Errorf("upstream message failed")
		}
	default:
		return fmt.Errorf("unsupported upstream stream protocol")
	}
	return nil
}

// googleCall 把 Google 一次性函数调用转换成统一工具增量。
// 参数 c 为包含函数 ID/名称/参数的 Turn；返回 toolDelta 写出错误；consume 的 Google 分支调用。
// 按已有调用身份去重，避免重复候选帧生成同一个工具两次；完整参数仍在 finish 校验。
// 修改工具状态并可能写出 SSE，不替调用方执行任何工具。
func (s *dialogueStream) googleCall(c llmdialogue.Turn) error {
	for _, existing := range s.calls {
		if existing.ID == c.ID {
			if existing.Name == c.Name && existing.Arguments == c.Arguments {
				return nil
			}
			return fmt.Errorf("conflicting Google function call ID")
		}
	}
	return s.toolDelta(len(s.calls), c.ID, c.Name, c.Arguments)
}

// PipeDialogue 将上游对话 SSE 解码为统一状态，再编码成调用方请求的协议。
// 参数 w 为客户端写入器，resp 为上游响应，start 为请求起点；alias 为公开模型，upstream/caller 为显式协议 ID；可选 beforeComplete 在有效终态发布前接收最终回复。
// 返回 wrote、usage、ttft、captured、err：是否实际写出、已观测用量、首数据耗时、拟发送帧捕获及错误。Serve 的适配流分支调用。
// 使用 SDK 的 SSE 帧解码器，不使用 SDK 客户端或重试；本函数负责关闭 resp.Body。
// 必须观察协议完成标记才调用 finish；EOF 无完成、供应商 error、非法工具参数或写出失败均不能作为成功回复。
// 已写出时尽力发协议失败事件并返回错误，执行器不得重试；未写出才可能安全尝试候选。
// 解码或验证失败不调用完成回调；若失败发生在回调后的终态写出，已保存的续接事实不会自动回滚。
// capture 不是客户端接收确认；状态和回调仅限单请求，不跨请求共享。
func PipeDialogue(w http.ResponseWriter, resp *http.Response, start time.Time, alias, upstream, caller string, beforeComplete ...func(llmdialogue.DialogueResult)) (bool, map[string]any, time.Duration, []byte, error) {
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	s := dialogueStream{writer: w, protocol: caller, model: alias, calls: map[int]*llmdialogue.Turn{}, blocks: map[int]int{}, usage: map[string]any{}, textBlock: -1, toolIndices: map[int]int{}}
	if len(beforeComplete) > 0 {
		s.beforeComplete = beforeComplete[0]
	}
	// 官方 SSE 分帧器处理注释、多行 data 和 CRLF；不接受 EOF 前未分隔的残缺事件。
	// 只复用解析器，没有创建 SDK 客户端，不会引入自动重试。
	decoder := ssestream.NewDecoder(resp)
	var ttft time.Duration
	var err error
	for decoder.Next() {
		err = s.consume(upstream, strings.TrimSpace(string(decoder.Event().Data)))
		if s.wrote && ttft == 0 {
			ttft = time.Since(start)
		}
		if err != nil {
			break
		}
	}
	if err == nil {
		err = decoder.Err()
	}
	if err == nil && !s.completed {
		err = fmt.Errorf("upstream stream ended without completion")
	}
	if err == nil {
		err = s.finish()
	}
	if err != nil && s.wrote { // 已输出后发送明确失败事件，不能生成完成事件或改用另一个部署。
		event := "error"
		payload := map[string]any{"type": "error", "error": map[string]any{"type": "upstream_error", "message": err.Error()}}
		if caller == "openai-responses" {
			event = "response.failed"
			payload = map[string]any{"type": event, "response": map[string]any{"id": s.id, "status": "failed", "error": map[string]any{"code": "upstream_error", "message": err.Error()}}}
		}
		_ = s.emit(event, payload)
	}
	return s.wrote, s.usage, ttft, s.capture.Bytes(), err
}
