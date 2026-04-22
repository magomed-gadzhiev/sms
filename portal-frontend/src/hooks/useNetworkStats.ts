import { useState, useCallback, useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { usePolling } from './usePolling';
import {
  networkStatsApi,
  type SharedFilter,
  type StatisticsResponse,
  type AnalyticsResponse,
  type MonitoringResponse,
  type DrillDownResponse,
  type SavedView,
} from '../api/networkStats';

type Mode = 'stats' | 'analytics' | 'monitoring';
type AnyResponse = StatisticsResponse | AnalyticsResponse | MonitoringResponse;

interface DrillDownLevel {
  sliceType: string;
  sliceValue: string;
  label: string;
}

const FILTER_KEYS: (keyof SharedFilter)[] = [
  'period_preset', 'date_from', 'date_to', 'group_by',
  'login', 'service_type', 'operator', 'channel',
  'sender_name', 'traffic_type', 'status', 'provider',
  'country', 'manager', 'error_code',
  'page', 'page_size', 'sort_by', 'sort_dir',
];

function parseFiltersFromURL(params: URLSearchParams): SharedFilter {
  const f: SharedFilter = {};
  for (const key of FILTER_KEYS) {
    const val = params.get(key);
    if (val) {
      if (key === 'page' || key === 'page_size') {
        (f as any)[key] = parseInt(val, 10) || undefined;
      } else {
        (f as any)[key] = val;
      }
    }
  }
  if (!f.period_preset && !f.date_from && !f.date_to) f.period_preset = '7d';
  if (!f.group_by) f.group_by = 'day';
  return f;
}

function filtersToParams(mode: Mode, filters: SharedFilter): Record<string, string> {
  const params: Record<string, string> = { mode };
  for (const [k, v] of Object.entries(filters)) {
    if (v !== undefined && v !== '' && v !== null) {
      params[k] = String(v);
    }
  }
  return params;
}

export function useNetworkStats() {
  const [searchParams, setSearchParams] = useSearchParams();

  const initialMode = (searchParams.get('mode') as Mode) || 'stats';
  const [mode, setModeState] = useState<Mode>(initialMode);
  const [filters, setFiltersState] = useState<SharedFilter>(() => parseFiltersFromURL(searchParams));
  const [data, setData] = useState<AnyResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Drill-down
  const [drillDown, setDrillDown] = useState<DrillDownResponse | null>(null);
  const [drillDownStack, setDrillDownStack] = useState<DrillDownLevel[]>([]);
  const [drillDownView, setDrillDownViewState] = useState('operators');
  const [drillDownLoading, setDrillDownLoading] = useState(false);

  // Saved views
  const [savedViews, setSavedViews] = useState<SavedView[]>([]);
  const [activeViewId, setActiveViewId] = useState<number | null>(null);
  const [isViewModified, setIsViewModified] = useState(false);

  // Export
  const [exportStatus, setExportStatus] = useState<{ status: string; job_id: string } | null>(null);

  // Abort controller
  const abortRef = useRef<AbortController | null>(null);

  // Keep a ref to latest filters & mode so fetchData always reads fresh values
  const filtersRef = useRef(filters);
  filtersRef.current = filters;
  const modeRef = useRef(mode);
  modeRef.current = mode;

  // Fetch data based on current mode (always reads latest filters via ref)
  const fetchData = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    const currentFilters = filtersRef.current;
    const currentMode = modeRef.current;

    setLoading(true);
    setError(null);
    try {
      let result: AnyResponse;
      switch (currentMode) {
        case 'analytics':
          result = await networkStatsApi.getAnalytics(currentFilters);
          break;
        case 'monitoring':
          result = await networkStatsApi.getMonitoring(currentFilters);
          break;
        default:
          result = await networkStatsApi.getStatistics(currentFilters);
      }
      if (!controller.signal.aborted) {
        setData(result);
      }
    } catch (err: any) {
      if (!controller.signal.aborted) {
        setError(err?.message || 'Ошибка загрузки данных');
      }
    } finally {
      if (!controller.signal.aborted) {
        setLoading(false);
      }
    }
  }, []);

  // Monitoring polling
  const polling = usePolling(fetchData, 10000);
  const isMonitoringMode = mode === 'monitoring';

  // Pause polling when not in monitoring mode.
  // IMPORTANT (D-18 fix): `polling` is a fresh object reference on every render,
  // so including it in deps made this effect fire on every render and
  // unconditionally re-resume polling — breaking user-initiated pause.
  // `polling.pause`/`polling.resume` are stable useCallbacks (wrapping stable setIsPaused),
  // so calling them through a stale polling ref is safe.
  useEffect(() => {
    if (isMonitoringMode) {
      polling.resume();
    } else {
      polling.pause();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isMonitoringMode]);

  // --- Actions ---

  const setMode = useCallback((newMode: Mode) => {
    setModeState(newMode);
    setSearchParams(filtersToParams(newMode, filters), { replace: true });
    setDrillDown(null);
    setDrillDownStack([]);
    // Auto-fetch data for the new mode
    setData(null);
  }, [filters, setSearchParams]);

  // Re-fetch when mode changes
  const prevModeRef = useRef(mode);
  useEffect(() => {
    if (prevModeRef.current !== mode) {
      prevModeRef.current = mode;
      fetchData();
    }
  }, [mode, fetchData]);

  const setFilters = useCallback((partial: Partial<SharedFilter>) => {
    setFiltersState(prev => {
      const next = { ...prev, ...partial };
      // Reset page to 1 when non-pagination filters change
      const nonPagKeys = Object.keys(partial).filter(k => k !== 'page' && k !== 'page_size');
      if (nonPagKeys.length > 0 && !('page' in partial)) {
        next.page = 1;
      }
      // Synchronously update ref so applyFilters() called immediately after reads fresh filters
      filtersRef.current = next;
      setIsViewModified(true);
      return next;
    });
  }, []);

  const applyFilters = useCallback(() => {
    setSearchParams(filtersToParams(modeRef.current, filtersRef.current), { replace: true });
    fetchData();
  }, [setSearchParams, fetchData]);

  // D-20 fix: toggle sort using filtersRef as the single source of truth for "current
  // filters". filtersRef is updated synchronously on every setFilters call (line ~170)
  // and on every render (line ~87), so it reflects the freshest state independent of
  // React commit timing / Suspense batching.
  const toggleSort = useCallback((key: string) => {
    const current = filtersRef.current;
    const newDir = current.sort_by === key && current.sort_dir === 'desc' ? 'asc' : 'desc';
    const next: SharedFilter = { ...current, sort_by: key, sort_dir: newDir, page: 1 };
    filtersRef.current = next;
    setFiltersState(next);
    setIsViewModified(true);
    applyFilters();
  }, [applyFilters]);

  // --- Drill-down ---

  const openDrillDown = useCallback(async (sliceType: string, sliceValue: string, label: string) => {
    setDrillDownLoading(true);
    setDrillDownStack([{ sliceType, sliceValue, label }]);
    setDrillDownView('operators');
    try {
      const result = await networkStatsApi.getDrillDown(filters, sliceType, sliceValue, 'operators');
      setDrillDown(result);
    } catch (err: any) {
      setError(err?.message || 'Ошибка загрузки детализации');
    } finally {
      setDrillDownLoading(false);
    }
  }, [filters]);

  const drillDeeper = useCallback(async (sliceType: string, sliceValue: string, label: string) => {
    setDrillDownLoading(true);
    const parentLevel = drillDownStack[drillDownStack.length - 1];
    setDrillDownStack(prev => [...prev, { sliceType, sliceValue, label }]);
    try {
      const result = await networkStatsApi.getDrillDown(
        filters, sliceType, sliceValue, 'operators',
        parentLevel?.sliceType, parentLevel?.sliceValue,
      );
      setDrillDown(result);
    } catch (err: any) {
      setError(err?.message || 'Ошибка загрузки детализации');
    } finally {
      setDrillDownLoading(false);
    }
  }, [filters, drillDownStack]);

  const navigateDrillDown = useCallback(async (level: number) => {
    if (level <= 0) {
      setDrillDown(null);
      setDrillDownStack([]);
      return;
    }
    const newStack = drillDownStack.slice(0, level);
    setDrillDownStack(newStack);
    setDrillDownLoading(true);
    const target = newStack[newStack.length - 1];
    const parent = newStack.length > 1 ? newStack[newStack.length - 2] : undefined;
    try {
      const result = await networkStatsApi.getDrillDown(
        filters, target.sliceType, target.sliceValue, drillDownView,
        parent?.sliceType, parent?.sliceValue,
      );
      setDrillDown(result);
    } catch (err: any) {
      setError(err?.message || 'Ошибка');
    } finally {
      setDrillDownLoading(false);
    }
  }, [drillDownStack, filters, drillDownView]);

  // Re-fetch drill-down data when the active tab (detail_view) changes
  const setDrillDownView = useCallback((view: string) => {
    setDrillDownViewState(view);
    // Re-fetch if we have an active drill-down
    const stack = drillDownStack;
    if (stack.length === 0) return;
    const target = stack[stack.length - 1];
    const parent = stack.length > 1 ? stack[stack.length - 2] : undefined;
    setDrillDownLoading(true);
    networkStatsApi.getDrillDown(
      filtersRef.current, target.sliceType, target.sliceValue, view,
      parent?.sliceType, parent?.sliceValue,
    ).then(result => {
      setDrillDown(result);
    }).catch((err: any) => {
      setError(err?.message || 'Ошибка загрузки детализации');
    }).finally(() => {
      setDrillDownLoading(false);
    });
  }, [drillDownStack]);

  const closeDrillDown = useCallback(() => {
    setDrillDown(null);
    setDrillDownStack([]);
  }, []);

  // --- Export ---

  const startExport = useCallback(async (format: 'csv' | 'xlsx') => {
    try {
      const resp = await networkStatsApi.startExport(filters, mode, format);
      setExportStatus({ status: 'pending', job_id: resp.job_id });

      const MAX_EXPORT_POLLS = 90; // 3 минуты при интервале 2с
      let pollCount = 0;

      const pollExport = async () => {
        pollCount++;
        if (pollCount > MAX_EXPORT_POLLS) {
          setExportStatus({ status: 'failed', job_id: resp.job_id });
          setError('Превышено время ожидания экспорта');
          return;
        }
        try {
          const status = await networkStatsApi.getExportStatus(resp.job_id);
          setExportStatus({ status: status.status, job_id: resp.job_id });
          if (status.status === 'done') {
            const res = await networkStatsApi.downloadExport(resp.job_id);
            if (res.ok) {
              const blob = await res.blob();
              const url = URL.createObjectURL(blob);
              const a = document.createElement('a');
              a.href = url;
              const disposition = res.headers.get('Content-Disposition');
              const match = disposition?.match(/filename="?([^"]+)"?/);
              a.download = match?.[1] || `export-${resp.job_id}`;
              document.body.appendChild(a);
              a.click();
              a.remove();
              URL.revokeObjectURL(url);
            }
          } else if (status.status === 'failed') {
            setError(status.error || 'Ошибка экспорта');
          } else {
            setTimeout(pollExport, 2000);
          }
        } catch {
          setExportStatus({ status: 'failed', job_id: resp.job_id });
          setError('Ошибка при проверке статуса экспорта');
        }
      };
      setTimeout(pollExport, 2000);
    } catch (err: any) {
      setError(err?.message || 'Ошибка экспорта');
    }
  }, [filters, mode]);

  // --- Saved views ---

  const [viewsError, setViewsError] = useState<string | null>(null);

  const loadViews = useCallback(async () => {
    setViewsError(null);
    try {
      const resp = await networkStatsApi.listViews();
      setSavedViews(resp.views || []);
    } catch (err: any) {
      setViewsError(err?.message || 'Ошибка загрузки сохранённых видов');
    }
  }, []);

  useEffect(() => { loadViews(); }, [loadViews]);

  const loadView = useCallback((id: number) => {
    const view = savedViews.find(v => v.id === id);
    if (!view) return;
    const parsedFilters: SharedFilter = view.filters_json ? JSON.parse(view.filters_json) : {};
    if (view.group_by) parsedFilters.group_by = view.group_by;
    if (view.sort_by) parsedFilters.sort_by = view.sort_by;
    if (view.sort_dir) parsedFilters.sort_dir = view.sort_dir;
    setFiltersState(parsedFilters);
    setModeState(view.mode as Mode);
    setActiveViewId(id);
    setIsViewModified(false);
    // Sync refs so fetchData reads fresh values when called synchronously below
    filtersRef.current = parsedFilters;
    modeRef.current = view.mode as Mode;
    setSearchParams(filtersToParams(view.mode as Mode, parsedFilters), { replace: true });
    // D-19 fix: explicitly trigger fetch. The mode-change useEffect only re-fetches
    // on mode change, not on filter-state change, so same-mode loadView would otherwise
    // leave the table showing stale data.
    fetchData();
  }, [savedViews, setSearchParams, fetchData]);

  const saveCurrentView = useCallback(async (name: string) => {
    try {
      const resp = await networkStatsApi.saveView({
        name,
        mode,
        filters_json: JSON.stringify(filters),
        group_by: filters.group_by || '',
        sort_by: filters.sort_by || '',
        sort_dir: filters.sort_dir || 'desc',
        columns: [],
        is_default: false,
      });
      setActiveViewId(resp.view.id);
      setIsViewModified(false);
      await loadViews();
    } catch (err: any) {
      setError(err?.message || 'Ошибка сохранения');
    }
  }, [mode, filters, loadViews]);

  const deleteViewById = useCallback(async (id: number) => {
    try {
      await networkStatsApi.deleteView(id);
      if (activeViewId === id) setActiveViewId(null);
      await loadViews();
    } catch (err: any) {
      setError(err?.message || 'Ошибка удаления');
    }
  }, [activeViewId, loadViews]);

  return {
    // State
    mode,
    filters,
    data,
    loading,
    error,

    // Actions
    setMode,
    setFilters,
    applyFilters,
    toggleSort,

    // Drill-down
    drillDown,
    drillDownStack,
    drillDownView,
    setDrillDownView,
    drillDownLoading,
    openDrillDown,
    drillDeeper,
    closeDrillDown,
    navigateDrillDown,

    // Saved views
    savedViews,
    activeViewId,
    isViewModified,
    viewsError,
    loadView,
    loadViews,
    saveCurrentView,
    deleteView: deleteViewById,

    // Export
    exportStatus,
    startExport,

    // Monitoring polling
    isPaused: polling.isPaused,
    togglePolling: polling.isPaused ? polling.resume : polling.pause,
    lastUpdated: polling.lastUpdated,
  };
}
