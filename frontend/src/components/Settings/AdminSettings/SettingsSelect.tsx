import React from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/cva.config";

/** 管理配置的原生下拉控件；参数为标准 select 属性，返回与项目 Input 一致的主题化控件。
 * 保留表单校验、键盘操作和 label 关联，无状态及网络副作用；禁用状态由调用方传入。
 */
export default function SettingsSelect({ className, ...props }: React.ComponentProps<"select">) {
  return <span className="relative block w-full"><select {...props} className={cn("h-9 w-full min-w-0 appearance-none rounded-md border border-input bg-background px-3 pr-8 text-sm shadow-xs outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30", className)} /><ChevronDown aria-hidden="true" className="pointer-events-none absolute right-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /></span>;
}
