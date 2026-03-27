interface WizardProgressProps {
  currentStep: number;
  totalSteps: number;
  labels: string[];
}

export function WizardProgress({ currentStep, totalSteps, labels }: WizardProgressProps) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', marginBottom: 32 }}>
      {Array.from({ length: totalSteps }, (_, i) => {
        const step = i + 1;
        const done = step < currentStep;
        const active = step === currentStep;
        return (
          <div key={step} style={{ display: 'flex', alignItems: 'center', flex: step < totalSteps ? 1 : undefined }}>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              <div
                style={{
                  width: 32,
                  height: 32,
                  borderRadius: '50%',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  background: done ? '#4caf50' : active ? '#1976d2' : '#e0e0e0',
                  color: done || active ? '#fff' : '#666',
                  fontWeight: 'bold',
                  fontSize: 14,
                }}
              >
                {done ? '✓' : step}
              </div>
              <div style={{ fontSize: 11, marginTop: 4, color: active ? '#1976d2' : '#666', whiteSpace: 'nowrap' }}>
                {labels[i]}
              </div>
            </div>
            {step < totalSteps && (
              <div
                style={{
                  flex: 1,
                  height: 2,
                  background: done ? '#4caf50' : '#e0e0e0',
                  margin: '0 4px',
                  marginBottom: 20,
                }}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
