import { Badge } from "@/components/ui/badge";
import { useDisableShowNewBadge } from "@/app/(dashboard)/hooks/useDisableShowNewBadge";
import { useT } from "@/i18n";

export default function BetaBadge({ children, dot = false }: { children?: React.ReactNode; dot?: boolean }) {
  const t = useT();
  const disableShowNewBadge = useDisableShowNewBadge();

  if (disableShowNewBadge) {
    return children ? <>{children}</> : null;
  }

  const badge = dot ? <Badge className="size-1.5 p-0" /> : <Badge>{t("Beta")}</Badge>;

  return children ? (
    <span className="inline-flex items-center gap-1.5">
      {children}
      {badge}
    </span>
  ) : (
    badge
  );
}
