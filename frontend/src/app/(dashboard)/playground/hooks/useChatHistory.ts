import React, { useState, useEffect } from "react";
import { useDebouncer } from "@tanstack/react-pacer/debouncer";
import { MessageType, A2ATaskMetadata } from "@/components/chat_ui/types";
import { TokenUsage } from "@/components/chat_ui/ResponseMetrics";
import { MCPEvent } from "@/components/mcp_tools/types";
import { truncateString } from "@/utils/textUtils";

const CHAT_HISTORY_PERSIST_WAIT_MS = 500;

/** 调试对话状态与更新动作；页面直接消费，明确会话 ID 与供应商响应游标分别管理。 */
export interface UseChatHistoryReturn {
  /** 客户端明确会话 UUID；首请求前生成、刷新恢复、清空更换，模型或端点切换保留。 */
  sessionId: string;
  /** 本轮工作区显示的消息列表；保存于当前浏览器会话，保留用户原文及用量信息。 */
  chatHistory: MessageType[];
  /** 直接替换或函数式更新消息列表；持久化由本 hook 统一延迟执行。 */
  setChatHistory: React.Dispatch<React.SetStateAction<MessageType[]>>;
  /** 当前对话的 MCP 工具事件，仅用于调试展示，不参与会话指纹。 */
  mcpEvents: MCPEvent[];
  /** 替换或追加工具事件，调用方负责使用当前请求的事件。 */
  setMCPEvents: React.Dispatch<React.SetStateAction<MCPEvent[]>>;
  /** 最近调用的日志追踪 ID，供日志跳转；不是会话 ID 或供应商响应游标。 */
  messageTraceId: string | null;
  /** 更新最近调用追踪 ID；普通工作区同步保存，null 清除。 */
  setMessageTraceId: React.Dispatch<React.SetStateAction<string | null>>;
  /** 上一成功 Responses 的响应 ID，供 previous_response_id 续接；不是会话 UUID，切换模型或端点清除。 */
  responsesSessionId: string | null;
  /** 更新上一成功响应的续接游标；失败请求不应写入成功游标，null 清除。 */
  setResponsesSessionId: React.Dispatch<React.SetStateAction<string | null>>;
  /** 是否通过 previous_response_id 续接 Responses；关闭后发送完整界面历史。 */
  useApiSessionManagement: boolean;
  /** 设置续接模式并保存；需要同时清理游标时使用 handleToggleSessionManagement。 */
  setUseApiSessionManagement: React.Dispatch<React.SetStateAction<boolean>>;

  /** 将文本分片追加到对应角色消息；model 仅在新消息首次创建时保存。 */
  updateTextUI: (role: string, chunk: string, model?: string) => void;
  /** 追加当前 assistant 消息的推理展示内容，不修改用户输入。 */
  updateReasoningContent: (chunk: string) => void;
  /** 保存当前 assistant 消息的首字延迟，参数单位毫秒。 */
  updateTimingData: (timeToFirstToken: number) => void;
  /** 保存归一化 token 用量；toolName 可标识本次工具相关统计。 */
  updateUsageData: (usage: TokenUsage, toolName?: string) => void;
  /** 保存当前消息关联的 A2A 任务状态，供页面展示。 */
  updateA2AMetadata: (a2aMetadata: A2ATaskMetadata) => void;
  /** 保存当前 assistant 消息的完整请求耗时，参数单位毫秒。 */
  updateTotalLatency: (totalLatency: number) => void;
  /** 保存当前消息的搜索结果列表；只影响调试展示。 */
  updateSearchResults: (searchResults: any[]) => void;
  /** 成功响应回调；启用 API 续接时更新供应商响应游标。 */
  handleResponseId: (responseId: string) => void;
  /** 切换续接模式；关闭 API 续接时清理游标，保留明确会话 ID。 */
  handleToggleSessionManagement: (useApi: boolean) => void;
  /** 处理工具事件并更新事件列表；普通工具往返保持会话身份。 */
  handleMCPEvent: (event: MCPEvent) => void;
  /** 追加生成图片消息并保留公开模型名称。 */
  updateImageUI: (imageUrl: string, model: string) => void;
  /** 追加截断展示的向量结果，不改写实际接口响应。 */
  updateEmbeddingsUI: (embeddings: string, model?: string) => void;
  /** 追加音频结果消息并保留公开模型名称。 */
  updateAudioUI: (audioUrl: string, model: string) => void;
  /** 向聊天结果追加图片，必要时创建 assistant 消息。 */
  updateChatImageUI: (imageUrl: string, model?: string) => void;
  /** 清空消息、游标、工具事件和追踪信息，取消待保存历史并立即更换会话 UUID。 */
  clearChatHistory: () => void;
  /** 只清理工具事件展示，保留聊天历史和会话 UUID。 */
  clearMCPEvents: () => void;
}

/** 管理调试对话与响应 ID；参数 simplified 禁用持久化，返回历史和更新动作；页面及密钥调试调用。
 * 恢复缓存时检查数据形状，损坏历史或模式回退默认值，避免旧缓存导致页面崩溃。 */
export function useChatHistory({ simplified }: { simplified: boolean }): UseChatHistoryReturn {
  // 首次渲染即创建并保存明确 ID；刷新复用，密钥调试保持独立且不持久化。
  const [sessionId, setSessionId] = useState(() => {
    const id = (!simplified && sessionStorage.getItem("playgroundSessionId")) || crypto.randomUUID();
    if (!simplified) sessionStorage.setItem("playgroundSessionId", id);
    return id;
  });
  const [chatHistory, setChatHistory] = useState<MessageType[]>(() => {
    if (simplified) return [];
    try {
      const saved = sessionStorage.getItem("chatHistory");
      const parsed: unknown = saved ? JSON.parse(saved) : [];
      return Array.isArray(parsed) &&
        parsed.every(
          (message) =>
            message &&
            typeof message.role === "string" &&
            (typeof message.content === "string" || Array.isArray(message.content)),
        )
        ? parsed
        : [];
    } catch (error) {
      console.error("Error parsing chatHistory from sessionStorage", error);
      return [];
    }
  });

  const [mcpEvents, setMCPEvents] = useState<MCPEvent[]>([]);

  const [messageTraceId, setMessageTraceId] = useState<string | null>(() =>
    simplified ? null : sessionStorage.getItem("messageTraceId") || null,
  );

  const [responsesSessionId, setResponsesSessionId] = useState<string | null>(() =>
    simplified ? null : sessionStorage.getItem("responsesSessionId") || null,
  );

  const [useApiSessionManagement, setUseApiSessionManagement] = useState<boolean>(() => {
    if (simplified) return true;
    try {
      const saved = sessionStorage.getItem("useApiSessionManagement");
      const parsed: unknown = saved ? JSON.parse(saved) : true;
      return typeof parsed === "boolean" ? parsed : true;
    } catch {
      return true;
    }
  });

  const persistDebouncer = useDebouncer(
    (history: MessageType[]) => {
      sessionStorage.setItem("chatHistory", JSON.stringify(history));
    },
    { wait: CHAT_HISTORY_PERSIST_WAIT_MS },
  );

  useEffect(() => {
    if (simplified || chatHistory.length === 0) {
      persistDebouncer.cancel();
      if (!simplified) sessionStorage.removeItem("chatHistory");
      return;
    }
    persistDebouncer.maybeExecute(chatHistory);
  }, [chatHistory, simplified, persistDebouncer]);

  // messageTraceId/responsesSessionId/useApiSessionManagement persistence
  useEffect(() => {
    if (simplified) return;
    if (messageTraceId) {
      sessionStorage.setItem("messageTraceId", messageTraceId);
    } else {
      sessionStorage.removeItem("messageTraceId");
    }
    if (responsesSessionId) {
      sessionStorage.setItem("responsesSessionId", responsesSessionId);
    } else {
      sessionStorage.removeItem("responsesSessionId");
    }
    sessionStorage.setItem("useApiSessionManagement", JSON.stringify(useApiSessionManagement));
  }, [messageTraceId, responsesSessionId, useApiSessionManagement, simplified]);

  const updateTextUI = (role: string, chunk: string, model?: string) => {
    setChatHistory((prev) => {
      const last = prev[prev.length - 1];
      // if the last message is already from this same role, append
      if (last && last.role === role && !last.isImage && !last.isAudio) {
        // build a new object, but only set `model` if it wasn't there already
        const updated: MessageType = {
          ...last,
          content: last.content + chunk,
          model: last.model ?? model, // ← only use the passed‐in model on the first chunk
        };
        return [...prev.slice(0, -1), updated];
      } else {
        // otherwise start a brand new assistant bubble
        return [
          ...prev,
          {
            role,
            content: chunk,
            model, // model set exactly once here
          },
        ];
      }
    });
  };

  const updateReasoningContent = (chunk: string) => {
    setChatHistory((prevHistory) => {
      const lastMessage = prevHistory[prevHistory.length - 1];

      if (lastMessage && lastMessage.role === "assistant" && !lastMessage.isImage && !lastMessage.isAudio) {
        return [
          ...prevHistory.slice(0, prevHistory.length - 1),
          {
            ...lastMessage,
            reasoningContent: (lastMessage.reasoningContent || "") + chunk,
          },
        ];
      } else {
        // If there's no assistant message yet, we'll create one with empty content
        // but with reasoning content
        if (prevHistory.length > 0 && prevHistory[prevHistory.length - 1].role === "user") {
          return [
            ...prevHistory,
            {
              role: "assistant",
              content: "",
              reasoningContent: chunk,
            },
          ];
        }

        return prevHistory;
      }
    });
  };

  const updateTimingData = (timeToFirstToken: number) => {
    setChatHistory((prevHistory) => {
      const lastMessage = prevHistory[prevHistory.length - 1];

      if (lastMessage && lastMessage.role === "assistant") {
        return [
          ...prevHistory.slice(0, prevHistory.length - 1),
          {
            ...lastMessage,
            timeToFirstToken,
          },
        ];
      }
      // If the last message is a user message and no assistant message exists yet,
      // create a new assistant message with empty content
      else if (lastMessage && lastMessage.role === "user") {
        return [
          ...prevHistory,
          {
            role: "assistant",
            content: "",
            timeToFirstToken,
          },
        ];
      }

      return prevHistory;
    });
  };

  const updateUsageData = (usage: TokenUsage, toolName?: string) => {
    setChatHistory((prevHistory) => {
      const lastMessage = prevHistory[prevHistory.length - 1];

      if (lastMessage && lastMessage.role === "assistant") {
        const updatedMessage = {
          ...lastMessage,
          usage,
          toolName,
        };

        return [...prevHistory.slice(0, prevHistory.length - 1), updatedMessage];
      }

      return prevHistory;
    });
  };

  const updateA2AMetadata = (a2aMetadata: A2ATaskMetadata) => {
    setChatHistory((prevHistory) => {
      const lastMessage = prevHistory[prevHistory.length - 1];

      if (lastMessage && lastMessage.role === "assistant") {
        const updatedMessage = {
          ...lastMessage,
          a2aMetadata,
        };
        return [...prevHistory.slice(0, prevHistory.length - 1), updatedMessage];
      }

      return prevHistory;
    });
  };

  const updateTotalLatency = (totalLatency: number) => {
    setChatHistory((prevHistory) => {
      const lastMessage = prevHistory[prevHistory.length - 1];

      if (lastMessage && lastMessage.role === "assistant") {
        return [
          ...prevHistory.slice(0, prevHistory.length - 1),
          {
            ...lastMessage,
            totalLatency,
          },
        ];
      }

      return prevHistory;
    });
  };

  const updateSearchResults = (searchResults: any[]) => {
    setChatHistory((prevHistory) => {
      const lastMessage = prevHistory[prevHistory.length - 1];

      if (lastMessage && lastMessage.role === "assistant") {
        const updatedMessage = {
          ...lastMessage,
          searchResults,
        };

        return [...prevHistory.slice(0, prevHistory.length - 1), updatedMessage];
      }

      return prevHistory;
    });
  };

  const handleResponseId = (responseId: string) => {
    if (useApiSessionManagement) {
      setResponsesSessionId(responseId);
    }
  };

  const handleToggleSessionManagement = (useApi: boolean) => {
    setUseApiSessionManagement(useApi);
    if (!useApi) {
      // Clear API session when switching to UI mode
      setResponsesSessionId(null);
    }
  };

  const handleMCPEvent = (event: MCPEvent) => {
    setMCPEvents((prev) => {
      // Check if this is a duplicate event (same item_id and type)
      // Only check for duplicates if item_id is defined (for mcp_list_tools, item_id is "mcp_list_tools")
      const isDuplicate = event.item_id
        ? prev.some(
            (existingEvent) =>
              existingEvent.item_id === event.item_id &&
              existingEvent.type === event.type &&
              (existingEvent.sequence_number === event.sequence_number ||
                (existingEvent.sequence_number === undefined && event.sequence_number === undefined)),
          )
        : false;

      if (isDuplicate) {
        return prev;
      }

      return [...prev, event];
    });
  };

  const updateImageUI = (imageUrl: string, model: string) => {
    setChatHistory((prevHistory) => [...prevHistory, { role: "assistant", content: imageUrl, model, isImage: true }]);
  };

  const updateEmbeddingsUI = (embeddings: string, model?: string) => {
    setChatHistory((prevHistory) => [
      ...prevHistory,
      { role: "assistant", content: truncateString(embeddings, 100), model, isEmbeddings: true },
    ]);
  };

  const updateAudioUI = (audioUrl: string, model: string) => {
    setChatHistory((prevHistory) => [...prevHistory, { role: "assistant", content: audioUrl, model, isAudio: true }]);
  };

  const updateChatImageUI = (imageUrl: string, model?: string) => {
    setChatHistory((prev) => {
      const last = prev[prev.length - 1];
      // If the last message is from assistant and has content, add image to it
      if (last && last.role === "assistant" && !last.isImage && !last.isAudio) {
        const updated = {
          ...last,
          image: {
            url: imageUrl,
            detail: "auto",
          },
          model: last.model ?? model,
        };
        return [...prev.slice(0, -1), updated];
      } else {
        // Otherwise create a new assistant message with just the image
        return [
          ...prev,
          {
            role: "assistant",
            content: "",
            model,
            image: {
              url: imageUrl,
              detail: "auto",
            },
          },
        ];
      }
    });
  };

  /** 清空历史、响应游标并更换会话 ID；无参数返回无，页面与切换身份调用。
   * 先取消待写历史，立即持久化新 ID，释放音频资源，避免清空后刷新恢复旧会话。 */
  const clearChatHistory = () => {
    persistDebouncer.cancel();
    const id = crypto.randomUUID();
    setSessionId(id);
    if (!simplified) sessionStorage.setItem("playgroundSessionId", id);
    // Use functional updater to get the latest snapshot — avoids stale-closure
    // bugs where audio messages added between the last render and the click
    // would leak their blob URLs.
    setChatHistory((prev) => {
      prev.forEach((message) => {
        if (message.isAudio && typeof message.content === "string") {
          URL.revokeObjectURL(message.content);
        }
      });
      return [];
    });

    setMessageTraceId(null);
    setResponsesSessionId(null); // Clear responses session ID
    setMCPEvents([]); // Clear MCP events
    if (!simplified) {
      sessionStorage.removeItem("chatHistory");
      sessionStorage.removeItem("messageTraceId");
      sessionStorage.removeItem("responsesSessionId");
    }
  };

  const clearMCPEvents = () => {
    setMCPEvents([]);
  };

  return {
    sessionId,
    chatHistory,
    setChatHistory,
    mcpEvents,
    setMCPEvents,
    messageTraceId,
    setMessageTraceId,
    responsesSessionId,
    setResponsesSessionId,
    useApiSessionManagement,
    setUseApiSessionManagement,
    updateTextUI,
    updateReasoningContent,
    updateTimingData,
    updateUsageData,
    updateA2AMetadata,
    updateTotalLatency,
    updateSearchResults,
    handleResponseId,
    handleToggleSessionManagement,
    handleMCPEvent,
    updateImageUI,
    updateEmbeddingsUI,
    updateAudioUI,
    updateChatImageUI,
    clearChatHistory,
    clearMCPEvents,
  };
}
