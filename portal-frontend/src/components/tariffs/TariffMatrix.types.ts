// Types specific to the TariffMatrix shared component.
// Re-exports TariffEditorData/TariffEditorCell from the API client (single source of truth,
// generated from the backend contract in Task 7).

import type { TariffEditorCell, TariffEditorData } from '../../api/client';

export type { TariffEditorCell, TariffEditorData };

export type TariffMatrixScope =
  | { kind: 'template'; templateId: string }
  | { kind: 'override'; subAccountId: string }
  | { kind: 'client' }; // client self-view, read-only, no inheritance markers

export interface TariffBulkSaveBody {
  period_id: string;
  tiers_upsert: { id: string | null; from_quantity: number; price_per_segment?: number }[];
  tiers_delete: string[];
  cells_upsert: {
    operator_id: string;
    tier_id: string;
    price: number;
    scope: 'template' | 'override';
    sub_account_id?: string;
  }[];
  cells_delete: {
    operator_id: string;
    tier_id: string;
    scope: 'template' | 'override';
    sub_account_id?: string;
  }[];
}

export interface TariffMatrixSaveError {
  operator_id: string;
  tier_id: string;
  reason: string;
}

export interface TariffMatrixSaveResult {
  ok: boolean;
  errors?: TariffMatrixSaveError[];
}

// The matrix always operates within a concrete period — callers must
// ensure `plan` and `active_period_id` are non-null before rendering.
export type TariffMatrixData = Omit<TariffEditorData, 'plan' | 'active_period_id'> & {
  plan: NonNullable<TariffEditorData['plan']>;
  active_period_id: string;
};

export interface TariffMatrixProps {
  data: TariffMatrixData;
  scope: TariffMatrixScope;
  /** false → no edit mode button/inputs */
  editable: boolean;
  /** true → mark template/override sources visually (●, italics, amber bg) */
  showInheritance: boolean;
  /** currency code for formatting (e.g. 'RUB' → '₽') */
  currency: string;
  onSaveBatch?: (batch: TariffBulkSaveBody) => Promise<TariffMatrixSaveResult>;
  /** bubble for page-level beforeunload guard */
  onUnsavedChange?: (hasChanges: boolean) => void;
}
