interface WizardNavProps {
  currentStep: number;
  totalSteps: number;
  onBack: () => void;
  onNext: () => void;
  onSubmit?: () => void;
  nextLabel?: string;
  submitLabel?: string;
  loading?: boolean;
  nextDisabled?: boolean;
}

export function WizardNav({
  currentStep,
  totalSteps,
  onBack,
  onNext,
  onSubmit,
  nextLabel,
  submitLabel,
  loading,
  nextDisabled,
}: WizardNavProps) {
  const isLast = currentStep === totalSteps;

  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 32 }}>
      <button
        type="button"
        onClick={onBack}
        disabled={currentStep === 1}
        style={{ padding: '8px 20px', cursor: currentStep === 1 ? 'not-allowed' : 'pointer' }}
      >
        ← Назад
      </button>
      <button
        type="button"
        onClick={isLast && onSubmit ? onSubmit : onNext}
        disabled={loading || nextDisabled}
        style={{
          padding: '8px 20px',
          background: '#1976d2',
          color: '#fff',
          border: 'none',
          borderRadius: 4,
          cursor: loading || nextDisabled ? 'not-allowed' : 'pointer',
        }}
      >
        {loading ? 'Сохраняем...' : (isLast ? (submitLabel ?? nextLabel ?? 'Создать') : (nextLabel ?? 'Далее →'))}
      </button>
    </div>
  );
}
