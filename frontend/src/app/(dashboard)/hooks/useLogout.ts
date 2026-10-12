import { clearTokenCookies } from "@/utils/cookieUtils";
import { clearStoredReturnUrl } from "@/utils/returnUrlUtils";
import useProxySettings from "@/app/(dashboard)/hooks/proxySettings/useProxySettings";
import { getProxyBaseUrl } from "@/components/networking";
import { getLoginUrl } from "@/utils/returnUrlUtils";
import { toast } from "@/lib/toast";
import { t } from "@/i18n";

/**
 * 生成真实退出处理器；accessToken 为当前会话 bearer，返回异步退出函数，供导航栏和侧栏调用。
 * 先撤销后台会话再清理本地凭据；失败时保留凭据以便重试，并显示国际化错误。
 */
export function useLogout(accessToken: string | null): () => Promise<void> {
  const proxySettings = useProxySettings(accessToken);

  return async () => {
    if (accessToken) {
      try {
        const response = await fetch(`${getProxyBaseUrl()}/auth/logout`, {
          method: "POST", headers: { Authorization: `Bearer ${accessToken}` },
        });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
      } catch {
        toast.error(t("session.logoutFailed"));
        return;
      }
    }
    clearTokenCookies();
    clearStoredReturnUrl();
    localStorage.removeItem("litellm_selected_worker_id");
    localStorage.removeItem("litellm_worker_url");
    window.location.href = proxySettings.PROXY_LOGOUT_URL || getLoginUrl();
  };
}
