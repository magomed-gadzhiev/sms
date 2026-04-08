export interface ConditionJSON {
  type: 'operator' | 'country' | 'traffic_type' | 'paid_name' | 'regex';
  value: string;
}

export interface ConditionGroupJSON {
  logic_op: 'IF' | 'AND' | 'AND_NOT' | 'OR' | 'OR_NOT';
  conditions: ConditionJSON[];
}

export interface ScheduleJSON {
  date_from?: string;
  date_to?: string;
  time_from?: string;
  time_to?: string;
  weekdays: number;
  timezone: string;
}

export interface RouteFormData {
  client_id?: string | null;
  name: string;
  comment: string;
  status: 'active' | 'draft';
  route_type: 'sms' | 'hlr' | 'max';
  operator_id?: string;
  provider_id: string;
  priority: number;
  share: number;
  condition_groups: ConditionGroupJSON[];
  schedules: ScheduleJSON[];
}

export interface RouteDetail {
  id: string;
  client_id: string | null;
  name: string;
  comment: string;
  status: string;
  route_type: string;
  operator_id?: string;
  provider_id: string;
  priority: number;
  share: number;
  condition_groups: ConditionGroupJSON[];
  schedules: ScheduleJSON[];
  created_at: string;
  updated_at: string;
}

export interface RouteListItem {
  id: string;
  client_id: string | null;
  name: string;
  status: string;
  route_type: string;
  provider_id: string;
  priority: number;
  share: number;
  comment: string;
  condition_tags: string[];
  schedule_summary: string;
  created_at: string;
  updated_at: string;
}

export interface RoutesListResponse {
  routes: RouteListItem[];
  total: number;
}

export interface RouteReferences {
  traffic_types: string[];
  route_types: string[];
  statuses: string[];
  logic_ops: string[];
  condition_types: string[];
}

export const WEEKDAY_LABELS = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'] as const;
export const WEEKDAY_BITS = [1, 2, 4, 8, 16, 32, 64] as const;

export const LOGIC_OP_LABELS: Record<string, string> = {
  IF: 'ЕСЛИ',
  AND: 'И',
  AND_NOT: 'И НЕ',
  OR: 'ИЛИ',
  OR_NOT: 'ИЛИ НЕ',
};

export const CONDITION_TYPE_LABELS: Record<string, string> = {
  operator: 'Оператор',
  country: 'Страна',
  traffic_type: 'Тип трафика',
  paid_name: 'Платное имя',
  regex: 'Regex',
};
