import type { NetworkBulkAssignResult } from '../../api/client';

interface ToastApi {
  success: (msg: string) => void;
  info: (msg: string) => void;
}

type AssignmentResult = Pick<NetworkBulkAssignResult, 'warnings'>;

/**
 * Унифицированный обработчик результата putAssignment / bulkAssign:
 * если бэкенд вернул warnings (частичный успех материализации) — toast.info
 * с описанием шагов; иначе toast.success.
 */
interface NotifyOptions {
  /** Текст toast.success, если warnings нет. По умолчанию: `${resourceLabel} обновлён`. */
  successMessage?: string;
  /**
   * Префикс для toast.info при частичной материализации.
   * По умолчанию: `${resourceLabel} сохранён, но материализация частично не удалась`.
   */
  partialPrefix?: string;
}

export function notifyAssignmentResult(
  toast: ToastApi,
  result: AssignmentResult,
  resourceLabel: string,
  options: NotifyOptions = {},
): void {
  if (result.warnings && result.warnings.length > 0) {
    const summary = result.warnings.map((w) => `${w.step}: ${w.error}`).join('; ');
    const prefix =
      options.partialPrefix ?? `${resourceLabel} сохранён, но материализация частично не удалась`;
    toast.info(`${prefix}: ${summary}. Повторим автоматически.`);
  } else {
    toast.success(options.successMessage ?? `${resourceLabel} обновлён`);
  }
}
