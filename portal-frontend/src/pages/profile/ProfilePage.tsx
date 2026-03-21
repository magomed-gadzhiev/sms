import { useState, useEffect, type FormEvent } from 'react';
import { profileApi, ApiError, type ProfileData } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';

export function ProfilePage() {
  const { user } = useAuth();
  const [profile, setProfile] = useState<ProfileData | null>(null);
  const [contactPerson, setContactPerson] = useState('');
  const [phone, setPhone] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState('');

  // 2FA state
  const [totpSetup, setTotpSetup] = useState<{ secret: string; qr_code_url: string } | null>(null);
  const [totpCode, setTotpCode] = useState('');
  const [disablePassword, setDisablePassword] = useState('');
  const [twoFaMsg, setTwoFaMsg] = useState('');

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
    try {
      const updated = await profileApi.update({ contact_person: contactPerson, phone });
      setProfile(updated);
      setSaveMsg('Profile updated');
    } catch (err) {
      setSaveMsg(err instanceof ApiError ? err.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  }

  async function handleSetupTOTP() {
    setTwoFaMsg('');
    try {
      const setup = await profileApi.setupTOTP();
      setTotpSetup(setup);
    } catch (err) {
      setTwoFaMsg(err instanceof ApiError ? err.message : 'Setup failed');
    }
  }

  async function handleVerifyTOTP(e: FormEvent) {
    e.preventDefault();
    setTwoFaMsg('');
    try {
      await profileApi.verifyTOTP(totpCode);
      setTotpSetup(null);
      setTotpCode('');
      const updated = await profileApi.get();
      setProfile(updated);
      setTwoFaMsg('2FA enabled successfully');
    } catch (err) {
      setTwoFaMsg(err instanceof ApiError ? err.message : 'Verification failed');
    }
  }

  async function handleDisableTOTP(e: FormEvent) {
    e.preventDefault();
    setTwoFaMsg('');
    try {
      await profileApi.disableTOTP(disablePassword);
      setDisablePassword('');
      const updated = await profileApi.get();
      setProfile(updated);
      setTwoFaMsg('2FA disabled');
    } catch (err) {
      setTwoFaMsg(err instanceof ApiError ? err.message : 'Failed to disable 2FA');
    }
  }

  if (!profile) return <div>Loading profile...</div>;

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
              <input type="tel" value={phone} onChange={(e) => setPhone(e.target.value)} />
            </label>
          </div>
          <button type="submit" disabled={saving}>
            {saving ? 'Saving...' : 'Save'}
          </button>
          {saveMsg && <span style={{ marginLeft: 12 }}>{saveMsg}</span>}
        </form>
      </section>

      <section>
        <h3>Two-Factor Authentication</h3>
        {twoFaMsg && <p>{twoFaMsg}</p>}

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
              Or enter the secret manually: <code>{totpSetup.secret}</code>
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
