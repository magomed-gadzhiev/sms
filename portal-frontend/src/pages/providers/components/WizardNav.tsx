interface WizardNavProps {
  currentStep: number;
  totalSteps: number;
  onBack: () => void;
  onNext: () => void;
  onSubmit?: () => void;
  nextLabel?: string;
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
        ← Back
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
        {loading ? 'Saving...' : (isLast ? (nextLabel ?? 'Create Provider') : (nextLabel ?? 'Next →'))}
      </button>
    </div>
  );
}
