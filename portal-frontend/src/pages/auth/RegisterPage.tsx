import { useState, type FormEvent } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { authApi, ApiError } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Input';

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
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-lg w-full p-8 bg-white rounded-lg shadow-sm border border-gray-200">
        <h2 className="text-2xl font-bold text-gray-900 mb-6">Create Account</h2>
        {error && (
          <p id="register-error" role="alert" className="text-sm text-danger bg-red-50 border border-red-200 rounded p-3 mb-4">
            {error}
          </p>
        )}
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Company Name"
            type="text"
            value={companyName}
            onChange={(e) => setCompanyName(e.target.value)}
            required
            autoFocus
            aria-describedby={error ? 'register-error' : undefined}
          />
          <Input
            label="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            aria-describedby={error ? 'register-error' : undefined}
          />
          <Input
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            placeholder="min 8 characters"
            minLength={8}
            aria-describedby={error ? 'register-error' : undefined}
          />
          <Input
            label="Contact Person"
            type="text"
            value={contactPerson}
            onChange={(e) => setContactPerson(e.target.value)}
          />
          <Input
            label="Phone"
            type="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
          />
          <fieldset className="border border-gray-300 rounded p-3">
            <legend className="text-sm font-medium text-gray-700 px-1">Plan</legend>
            <div className="flex gap-3 flex-wrap mt-1">
              {PLANS.map((plan) => (
                <label
                  key={plan.name}
                  className={`flex items-center gap-1.5 px-3 py-1.5 border-2 rounded cursor-pointer text-sm transition-colors
                    ${planName === plan.name
                      ? 'border-primary bg-blue-50 font-semibold text-primary'
                      : 'border-gray-300 hover:border-gray-400 text-gray-700'}`}
                >
                  <input
                    type="radio"
                    name="plan"
                    value={plan.name}
                    checked={planName === plan.name}
                    onChange={() => setPlanName(plan.name)}
                    className="sr-only"
                  />
                  <span>{plan.label}</span>
                  <span className="text-xs text-gray-500">{plan.price}</span>
                </label>
              ))}
            </div>
          </fieldset>
          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? 'Registering...' : 'Register'}
          </Button>
        </form>
        <p className="mt-4 text-sm text-gray-600">
          Already have an account?{' '}
          <Link to="/login" className="text-primary hover:text-primary-dark">Login</Link>
        </p>
      </div>
    </div>
  );
}
