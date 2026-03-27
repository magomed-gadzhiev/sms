import { useState, type FormEvent } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { authApi, ApiError } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';

const PLANS = [
  { name: 'free', label: 'Free', price: '0₽' },
  { name: 'starter', label: 'Starter', price: '5 000₽' },
  { name: 'business', label: 'Business', price: '15 000₽' },
  { name: 'pro', label: 'Pro', price: '40 000₽' },
] as const;

export function RegisterPage() {
  const { isAuthenticated, refreshUser } = useAuth();
  const navigate = useNavigate();

  const [companyName, setCompanyName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [contactPerson, setContactPerson] = useState('');
  const [phone, setPhone] = useState('');
  const [planName, setPlanName] = useState<string>('free');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  if (isAuthenticated) {
    navigate('/dashboard', { replace: true });
    return null;
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      await authApi.register({
        email,
        password,
        company_name: companyName,
        ...(contactPerson ? { contact_person: contactPerson } : {}),
        ...(phone ? { phone } : {}),
        plan_name: planName,
      });
      await refreshUser();
      navigate('/dashboard', { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Registration failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div style={{ maxWidth: 480, margin: '60px auto' }}>
      <h2>Create Account</h2>
      {error && (
        <p id="register-error" role="alert" style={{ color: '#d32f2f' }}>
          {error}
        </p>
      )}
      <form onSubmit={handleSubmit}>
        <div style={{ marginBottom: 12 }}>
          <label>
            Company Name *
            <br />
            <input
              type="text"
              value={companyName}
              onChange={(e) => setCompanyName(e.target.value)}
              required
              autoFocus
              aria-describedby={error ? 'register-error' : undefined}
            />
          </label>
        </div>
        <div style={{ marginBottom: 12 }}>
          <label>
            Email *
            <br />
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              aria-describedby={error ? 'register-error' : undefined}
            />
          </label>
        </div>
        <div style={{ marginBottom: 12 }}>
          <label>
            Password * (min 8 characters)
            <br />
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
              aria-describedby={error ? 'register-error' : undefined}
            />
          </label>
        </div>
        <div style={{ marginBottom: 12 }}>
          <label>
            Contact Person
            <br />
            <input
              type="text"
              value={contactPerson}
              onChange={(e) => setContactPerson(e.target.value)}
            />
          </label>
        </div>
        <div style={{ marginBottom: 12 }}>
          <label>
            Phone
            <br />
            <input
              type="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
            />
          </label>
        </div>
        <div style={{ marginBottom: 16 }}>
          <fieldset style={{ border: '1px solid #ddd', borderRadius: 4, padding: '8px 12px' }}>
            <legend>Plan</legend>
            <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', marginTop: 8 }}>
              {PLANS.map((plan) => (
                <label
                  key={plan.name}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 6,
                    padding: '6px 12px',
                    border: `2px solid ${planName === plan.name ? '#1976d2' : '#ddd'}`,
                    borderRadius: 4,
                    cursor: 'pointer',
                    fontWeight: planName === plan.name ? 'bold' : 'normal',
                  }}
                >
                  <input
                    type="radio"
                    name="plan"
                    value={plan.name}
                    checked={planName === plan.name}
                    onChange={() => setPlanName(plan.name)}
                    style={{ display: 'none' }}
                  />
                  <span>{plan.label}</span>
                  <span style={{ color: '#666', fontSize: 13 }}>{plan.price}</span>
                </label>
              ))}
            </div>
          </fieldset>
        </div>
        <button type="submit" disabled={submitting}>
          {submitting ? 'Registering...' : 'Register'}
        </button>
      </form>
      <p style={{ marginTop: 16 }}>
        Already have an account? <Link to="/login">Login</Link>
      </p>
    </div>
  );
}
