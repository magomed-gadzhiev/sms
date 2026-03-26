import { useState, useEffect, type FormEvent } from 'react';
import { profileApi, ApiError, type ProfileData } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';

export function ProfilePage() {
  useAuth();
  const [profile, setProfile] = useState<ProfileData | null>(null);
  const [contactPerson, setContactPerson] = useState('');
  const [phone, setPhone] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState('');
  const [saveIsError, setSaveIsError] = useState(false);

  // 2FA state
  const [totpSetup, setTotpSetup] = useState<{ secret: string; qr_code_url: string } | null>(null);
  const [totpCode, setTotpCode] = useState('');
  const [disablePassword, setDisablePassword] = useState('');
  const [twoFaMsg, setTwoFaMsg] = useState('');
  const [twoFaIsError, setTwoFaIsError] = useState(false);

  useEffect(() => {
    profileApi.get().then((p) => {
      setProfile(p);
      setContactPerson(p.contact_person);
      setPhone(p.phone);
    });
  }, []);

  async function handleSaveProfile(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setSaveMsg('');
    setSaveIsError(false);
    try {
      const updated = await profileApi.update({ contact_person: contactPerson, phone });
      setProfile(updated);
      setSaveMsg('Profile updated');
      setSaveIsError(false);
    } catch (err) {
      setSaveMsg(err instanceof ApiError ? err.message : 'Save failed');
      setSaveIsError(true);
    } finally {
      setSaving(false);
    }
  }

  async function handleSetupTOTP() {
    setTwoFaMsg('');
    setTwoFaIsError(false);
    try {
      const setup = await profileApi.setupTOTP();
      setTotpSetup(setup);
    } catch (err) {
      setTwoFaMsg(err instanceof ApiError ? err.message : 'Setup failed');
      setTwoFaIsError(true);
    }
  }

  async function handleVerifyTOTP(e: FormEvent) {
    e.preventDefault();
    setTwoFaMsg('');
    setTwoFaIsError(false);
    try {
      await profileApi.verifyTOTP(totpCode);
      setTotpSetup(null);
      setTotpCode('');
      const updated = await profileApi.get();
      setProfile(updated);
      setTwoFaMsg('2FA enabled successfully');
      setTwoFaIsError(false);
    } catch (err) {
      setTwoFaMsg(err instanceof ApiError ? err.message : 'Verification failed');
      setTwoFaIsError(true);
    }
  }

  async function handleDisableTOTP(e: FormEvent) {
    e.preventDefault();
    setTwoFaMsg('');
    setTwoFaIsError(false);
    try {
      await profileApi.disableTOTP(disablePassword);
      setDisablePassword('');
      const updated = await profileApi.get();
      setProfile(updated);
      setTwoFaMsg('2FA disabled');
      setTwoFaIsError(false);
    } catch (err) {
      setTwoFaMsg(err instanceof ApiError ? err.message : 'Failed to disable 2FA');
      setTwoFaIsError(true);
    }
  }

  if (!profile) return <div role="status">Loading profile...</div>;

  return (
    <div style={{ maxWidth: 600 }}>
      <h2>Profile</h2>

      <section style={{ marginBottom: 32 }}>
        <p>
          <strong>Email:</strong> {profile.email}
        </p>
        <p>
          <strong>Company:</strong> {profile.company_name}
        </p>

        <form onSubmit={handleSaveProfile}>
          {saveMsg && saveIsError && (
            <p id="profile-error" role="alert" style={{ color: '#d32f2f' }}>{saveMsg}</p>
          )}
          <div style={{ marginBottom: 12 }}>
            <label>
              Contact Person
              <br />
              <input
                type="text"
                value={contactPerson}
                onChange={(e) => setContactPerson(e.target.value)}
                aria-describedby={saveMsg && saveIsError ? 'profile-error' : undefined}
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
                aria-describedby={saveMsg && saveIsError ? 'profile-error' : undefined}
              />
            </label>
          </div>
          <button type="submit" disabled={saving}>
            {saving ? 'Saving...' : 'Save'}
          </button>
          {saveMsg && !saveIsError && (
            <p role="status" style={{ display: 'inline', marginLeft: 12 }}>{saveMsg}</p>
          )}
        </form>
      </section>

      <section>
        <h3>Two-Factor Authentication</h3>
        {twoFaMsg && twoFaIsError && (
          <p id="profile-2fa-error" role="alert" style={{ color: '#d32f2f' }}>{twoFaMsg}</p>
        )}
        {twoFaMsg && !twoFaIsError && (
          <p role="status">{twoFaMsg}</p>
        )}

        {profile.totp_enabled ? (
          <div>
            <p>2FA is currently enabled.</p>
            <form onSubmit={handleDisableTOTP}>
              <div style={{ marginBottom: 12 }}>
                <label>
                  Enter password to disable 2FA
                  <br />
                  <input
                    type="password"
                    value={disablePassword}
                    onChange={(e) => setDisablePassword(e.target.value)}
                    required
                  />
                </label>
              </div>
              <button type="submit">Disable 2FA</button>
            </form>
          </div>
        ) : totpSetup ? (
          <div>
            <p>Scan the QR code with your authenticator app:</p>
            <img src={totpSetup.qr_code_url} alt="TOTP QR Code" style={{ maxWidth: 200 }} />
            <p>
              Or enter the secret manually:{' '}
              <code>{totpSetup.secret}</code>
              {' '}
              <button
                type="button"
                onClick={() => navigator.clipboard.writeText(totpSetup.secret)}
                style={{ marginLeft: 8 }}
              >
                Copy secret
              </button>
            </p>
            <form onSubmit={handleVerifyTOTP}>
              <div style={{ marginBottom: 12 }}>
                <label>
                  Verification Code
                  <br />
                  <input
                    type="text"
                    value={totpCode}
                    onChange={(e) => setTotpCode(e.target.value)}
                    required
                    inputMode="numeric"
                    pattern="[0-9]{6}"
                    maxLength={6}
                  />
                </label>
              </div>
              <button type="submit">Verify &amp; Enable</button>
            </form>
          </div>
        ) : (
          <div>
            <p>2FA is not enabled.</p>
            <button onClick={handleSetupTOTP}>Set up 2FA</button>
          </div>
        )}
      </section>
    </div>
  );
}
