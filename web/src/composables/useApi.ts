import { ElMessage } from 'element-plus'

/**
 * API 错误处理 composable
 * 统一封装 axios 错误处理逻辑
 */
export function useApi() {
  /**
   * 处理 API 错误并显示消息
   * @param error - 错误对象 (通常是 axios error)
   * @param fallbackMsg - 备用错误消息
   */
  function handleError(error: any, fallbackMsg = '操作失败'): string {
    const msg = error?.response?.data?.error || error?.message || fallbackMsg
    ElMessage.error(msg)
    return msg
  }

  /**
   * 处理 API 错误但不显示消息 (用于静默失败场景)
   */
  function handleErrorSilent(error: any): string | null {
    if (!error) return null
    const msg = error?.response?.data?.error || error?.message || null
    return msg
  }

  /**
   * 显示成功消息
   */
  function showSuccess(msg = '操作成功') {
    ElMessage.success(msg)
  }

  /**
   * 显示警告消息
   */
  function showWarning(msg: string) {
    ElMessage.warning(msg)
  }

  return {
    handleError,
    handleErrorSilent,
    showSuccess,
    showWarning,
  }
}