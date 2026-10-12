import React from "react";
import { Control, UseFormReturn } from "react-hook-form";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { CircleHelp } from "lucide-react";
import { FormField } from "@/components/shared/form/FormField";
import { toast } from "@/lib/toast";
import AgentSelector from "../agent_management/AgentSelector";
import NumericalInput from "../shared/numerical_input";
import SkillSelector from "../skills/SkillSelector";
import { moveTagsOutOfMetadataJson } from "./keyEditFieldNormalizers";
import { AgentsAndGroups, KeyEditFormValues } from "./keyEditFormValues";
import { useT, t } from "@/i18n";

export const labelWithHint = (label: React.ReactNode, hint: string): React.ReactNode => (
  <>
    {label}
    <Tooltip>
      <TooltipTrigger render={<CircleHelp className="size-3.5 shrink-0 cursor-help text-muted-foreground" />} />
      <TooltipContent className="max-w-xs">{hint}</TooltipContent>
    </Tooltip>
  </>
);

const KEY_TYPE_OPTIONS = [
  { value: "default", /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Full Access"); }, /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get hint() { return t("Can call all routes (AI APIs, Management, and read-only)"); } },
  { value: "llm_api", /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("AI APIs"); }, /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get hint() { return t("Can call only AI API routes (chat/completions, embeddings, etc.)"); } },
  { value: "management", /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Management"); }, /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get hint() { return t("Can call only management routes (user/team/key management)"); } },
];

export const KeyTypeSelect = ({
  id,
  value,
  onChange,
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
}) => { const t = useT(); return ((
  <Select
    items={Object.fromEntries(KEY_TYPE_OPTIONS.map((option) => [option.value, option.label]))}
    value={value}
    onValueChange={(next: string | null) => next != null && onChange(next)}
  >
    <SelectTrigger id={id} className="w-full">
      <SelectValue placeholder={t("Select key type")} />
    </SelectTrigger>
    <SelectContent>
      {KEY_TYPE_OPTIONS.map((option) => (
        <SelectItem key={option.value} value={option.value}>
          <div className="py-1">
            <div className="font-medium">{option.label}</div>
            <div className="mt-0.5 text-[11px] text-muted-foreground">{option.hint}</div>
          </div>
        </SelectItem>
      ))}
    </SelectContent>
  </Select>
)); };

const SKILLS_HINT =
  "Enabled skills are visible to every key. Grant disabled (private) Claude Code plugins to this key here.";

export const KeyAgentAndSkillFields = ({
  control,
  accessToken,
}: {
  control: Control<KeyEditFormValues>;
  accessToken: string;
}) => { const t = useT(); return ((
  <>
    <FormField control={control} name="agents_and_groups" label={t("Agents / Access Groups")}>
      {({ value, onChange }) => (
        <AgentSelector
          onChange={onChange}
          value={value as AgentsAndGroups | undefined}
          accessToken={accessToken}
          placeholder={t("Select agents or access groups (optional)")}
        />
      )}
    </FormField>

    <FormField control={control} name="skills" label={labelWithHint(t("Skills"), SKILLS_HINT)}>
      {({ value, onChange }) => (
        <SkillSelector onChange={onChange} value={value as string[] | undefined} accessToken={accessToken} />
      )}
    </FormField>
  </>
)); };

type KeyEditForm = Pick<
  UseFormReturn<KeyEditFormValues, unknown, KeyEditFormValues>,
  "control" | "getValues" | "setValue"
>;

export const moveMetadataTagsToTagsField = (form: KeyEditForm): void => {
  const moved = moveTagsOutOfMetadataJson(form.getValues("metadata"), form.getValues("tags"));
  if (moved === null) return;
  form.setValue("metadata", moved.metadata, { shouldDirty: true });
  form.setValue("tags", moved.tags, { shouldDirty: true });
  if (moved.movedTags.length > 0) {
    toast.info(t("Moved tags from metadata into the Tags field: {value0}", { value0: (moved.movedTags.join(", ")) }));
  }
};

export const KeyMetadataField = ({ form }: { form: KeyEditForm }) => { const t = useT(); return ((
  <FormField
    control={form.control}
    name="metadata"
    label={t("Metadata")}
    description={t("Optional JSON. A tags array is moved into the Tags field when you leave this box.")}
  >
    {(field) => (
      <Textarea
        {...field}
        value={(field.value as string | undefined) ?? ""}
        rows={10}
        onBlur={() => {
          field.onBlur();
          moveMetadataTagsToTagsField(form);
        }}
      />
    )}
  </FormField>
)); };

export const KeyBudgetNumberField = ({
  control,
  name,
  label,
  placeholder,
}: {
  control: Control<KeyEditFormValues>;
  name: "max_budget" | "soft_budget";
  label: string;
  placeholder: string;
}) => { const t = useT(); return ((
  <FormField control={control} name={name} label={label}>
    {({ ref: _ref, ...field }) => (
      <NumericalInput
        {...field}
        value={field.value ?? ""}
        step={0.01}
        style={{ width: "100%" }}
        placeholder={placeholder}
      />
    )}
  </FormField>
)); };
