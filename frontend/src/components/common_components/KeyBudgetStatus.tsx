import { useT } from "@/i18n";

/** 预算占用的授权详情；ID 用于管理员核对，状态由后台持久化，不提供自动释放按钮。 */
export interface BudgetReservationView {
  reservation_id: string;
  amount: number;
  state: string;
  reason?: string;
}

/** 展示有限额调用的公开契约和待结算占用；参数为服务端金额/尝试，无副作用。
 * 未知状态按待核对显示；动态金额和标识原样保留，所有说明和状态使用当前界面语言。 */
export function KeyBudgetStatus({
  pending = 0,
  reservations = [],
}: {
  pending?: number;
  reservations?: BudgetReservationView[] | null;
}) {
  const t = useT();
  const rows = reservations ?? [];
  const states = new Set(["reserved", "submitted", "settling", "uncertain"]);
  return (
    <div className="space-y-2 text-sm" aria-label={t("hardBudget.details")}>
      <p className="text-muted-foreground">{t("hardBudget.contract", { cap: 4096 })}</p>
      <p>{t("hardBudget.pending", { amount: pending })}</p>
      {rows.length > 0 && (
        <>
          <p className="text-muted-foreground">{t("hardBudget.reconcile")}</p>
          <ul>
            {rows.map((item) => (
              <li key={item.reservation_id}>
                <code>{item.reservation_id}</code>
                {" — "}
                {t("hardBudget.attempt", {
                  amount: item.amount,
                  state: t("hardBudget.states." + (states.has(item.state) ? item.state : "uncertain")),
                })}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
