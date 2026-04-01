interface StepConfig {
  key: string;
  label: string;
}

interface Props {
  steps: StepConfig[];
  currentIndex: number;
  maxReachedIndex: number;
  validationErrors?: Record<string, boolean>;
  onStepClick: (index: number) => void;
}

export function StepIndicator({ steps, currentIndex, maxReachedIndex, validationErrors = {}, onStepClick }: Props) {
  return (
    <nav aria-label="Шаги создания рассылки">
      <ol className="flex items-center gap-2">
        {steps.map((step, idx) => {
          const isActive = idx === currentIndex;
          const isCompleted = idx < currentIndex;
          const isReachable = idx <= maxReachedIndex;
          const hasError = validationErrors[step.key];

          const stepStatus = isActive ? 'текущий' : isCompleted ? 'завершён' : 'ожидает';

          return (
            <li key={step.key} className="flex items-center gap-2">
              {idx > 0 && (
                <div
                  className={`w-8 h-0.5 ${isCompleted ? 'bg-blue-500' : 'bg-gray-300'}`}
                  aria-hidden="true"
                />
              )}
              <div className="relative">
                <button
                  onClick={() => isReachable && onStepClick(idx)}
                  disabled={!isReachable}
                  aria-label={`${step.label} — ${stepStatus}`}
                  aria-current={isActive ? 'step' : undefined}
                  className={`px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-blue-100 text-blue-700'
                      : isCompleted
                        ? 'bg-green-100 text-green-700 cursor-pointer hover:bg-green-200'
                        : isReachable
                          ? 'bg-gray-100 text-gray-700 cursor-pointer hover:bg-gray-200'
                          : 'bg-gray-100 text-gray-400 cursor-not-allowed'
                  }`}
                >
                  {step.label}
                </button>
                {hasError && (
                  <span
                    className="absolute -top-1 -right-1 w-3 h-3 bg-red-500 rounded-full border-2 border-white"
                    aria-label="Ошибка валидации"
                  />
                )}
              </div>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
