import { useState, useEffect, type FormEvent } from 'react';
import { profileApi, ApiError, type ProfileData } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import { PageHeader } from '../../components/layout/PageHeader';
import { Input } from '../../components/ui/Input';
import { Button } from '../../components/ui/Button';

export function ProfilePage() {
  const { refreshUser } = useAuth();
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

  // Change password state
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [pwSaving, setPwSaving] = useState(false);
  const [pwMsg, setPwMsg] = useState('');
  const [pwIsError, setPwIsError] = useState(false);

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
      await refreshUser();
      setSaveMsg('Профиль обновлён');
      setSaveIsError(false);
    } catch (err) {
      setSaveMsg(err instanceof ApiError ? err.message : 'Не удалось сохранить');
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
      setTwoFaMsg(err instanceof ApiError ? err.message : 'Ошибка настройки');
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
      setTwoFaMsg('2FA успешно включена');
      setTwoFaIsError(false);
    } catch (err) {
      setTwoFaMsg(err instanceof ApiError ? err.message : 'Ошибка подтверждения');
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
      setTwoFaMsg('2FA отключена');
      setTwoFaIsError(false);
    } catch (err) {
      setTwoFaMsg(err instanceof ApiError ? err.message : 'Не удалось отключить 2FA');
      setTwoFaIsError(true);
    }
  }

  async function handleChangePassword(e: FormEvent) {
    e.preventDefault();
    setPwMsg('');
    setPwIsError(false);

    if (newPassword.length < 8) {
      setPwMsg('Новый пароль должен содержать минимум 8 символов');
      setPwIsError(true);
      return;
    }
    if (newPassword !== confirmPassword) {
      setPwMsg('Пароли не совпадают');
      setPwIsError(true);
      return;
    }

    setPwSaving(true);
    try {
      await profileApi.changePassword(currentPassword, newPassword);
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setPwMsg('Пароль успешно изменен');
      setPwIsError(false);
    } catch (err) {
      setPwMsg(err instanceof ApiError ? err.message : 'Ошибка при смене пароля');
      setPwIsError(true);
    } finally {
      setPwSaving(false);
    }
  }

  if (!profile) return <div role="status">Загрузка профиля...</div>;

  return (
    <div className="max-w-xl">
      <PageHeader title="Профиль" />

      <section className="mb-8">
        <p className="text-gray-700"><strong>Email:</strong> {profile.email}</p>
        <p className="text-gray-700"><strong>Компания:</strong> {profile.company_name}</p>

        <form onSubmit={handleSaveProfile} className="mt-4">
          {saveMsg && saveIsError && (
            <p id="profile-error" role="alert" className="text-red-600 mb-3">{saveMsg}</p>
          )}
          <div className="mb-2">
            <Input
              label="Контактное лицо"
              value={contactPerson}
              onChange={(e) => setContactPerson(e.target.value)}
              maxLength={255}
              autoComplete="name"
              aria-describedby={saveMsg && saveIsError ? 'profile-error' : undefined}
            />
          </div>
          <div className="mb-2">
            <Input
              label="Телефон"
              type="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              maxLength={50}
              autoComplete="tel"
              aria-describedby={saveMsg && saveIsError ? 'profile-error' : undefined}
            />
          </div>
          <div className="flex items-center gap-3">
            <Button type="submit" disabled={saving}>
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
            {saveMsg && !saveIsError && (
              <p role="status" className="inline text-green-600">{saveMsg}</p>
            )}
          </div>
        </form>
      </section>

      <section className="border border-gray-200 rounded-lg p-6">
        <h3 className="text-lg font-semibold mb-4">Двухфакторная аутентификация</h3>
        {twoFaMsg && twoFaIsError && (
          <p id="profile-2fa-error" role="alert" className="text-red-600 mb-3">{twoFaMsg}</p>
        )}
        {twoFaMsg && !twoFaIsError && (
          <p role="status" className="text-green-600 mb-3">{twoFaMsg}</p>
        )}

        {profile.totp_enabled ? (
          <div>
            <p className="mb-3">2FA включена.</p>
            <form onSubmit={handleDisableTOTP}>
              <div className="mb-3">
                <Input
                  label="Введите пароль для отключения 2FA"
                  type="password"
                  value={disablePassword}
                  onChange={(e) => setDisablePassword(e.target.value)}
                  required
                  autoComplete="current-password"
                />
              </div>
              <Button type="submit" variant="danger">Отключить 2FA</Button>
            </form>
          </div>
        ) : totpSetup ? (
          <div>
            <p className="mb-3">Отсканируйте QR-код приложением-аутентификатором:</p>
            <div className="bg-gray-50 rounded-lg p-4 mb-4 inline-block">
              <img src={totpSetup.qr_code_url} alt="TOTP QR Code" className="max-w-[200px]" />
            </div>
            <p className="mb-3">
              Или введите секрет вручную:{' '}
              <code className="bg-white px-2 py-1 rounded border text-sm">{totpSetup.secret}</code>
              {' '}
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={() => navigator.clipboard.writeText(totpSetup.secret)}
                className="ml-2"
              >
                Скопировать секрет
              </Button>
            </p>
            <form onSubmit={handleVerifyTOTP}>
              <div className="mb-3">
                <Input
                  label="Код подтверждения"
                  value={totpCode}
                  onChange={(e) => setTotpCode(e.target.value)}
                  required
                  inputMode="numeric"
                  pattern="[0-9]{6}"
                  maxLength={6}
                />
              </div>
              <Button type="submit">Подтвердить и включить</Button>
            </form>
          </div>
        ) : (
          <div>
            <p className="mb-3">2FA не включена.</p>
            <Button onClick={handleSetupTOTP}>Настроить 2FA</Button>
          </div>
        )}
      </section>

      <section className="border border-gray-200 rounded-lg p-6 mt-8">
        <h3 className="text-lg font-semibold mb-4">Сменить пароль</h3>

        {pwMsg && pwIsError && (
          <p id="pw-error" role="alert" className="text-red-600 mb-3">{pwMsg}</p>
        )}
        {pwMsg && !pwIsError && (
          <p role="status" className="text-green-600 mb-3">{pwMsg}</p>
        )}

        <form onSubmit={handleChangePassword}>
          <div className="mb-3">
            <Input
              label="Текущий пароль"
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              required
              autoComplete="current-password"
              aria-describedby={pwMsg && pwIsError ? 'pw-error' : undefined}
            />
          </div>
          <div className="mb-3">
            <Input
              label="Новый пароль"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
              autoComplete="new-password"
              aria-describedby={pwMsg && pwIsError ? 'pw-error' : undefined}
            />
            <p className="text-xs text-gray-500 mt-1">Минимум 8 символов</p>
          </div>
          <div className="mb-4">
            <Input
              label="Подтверждение пароля"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              autoComplete="new-password"
              aria-describedby={pwMsg && pwIsError ? 'pw-error' : undefined}
            />
          </div>
          <Button type="submit" disabled={pwSaving}>
            {pwSaving ? 'Сохранение...' : 'Сменить пароль'}
          </Button>
        </form>
      </section>
    </div>
  );
}
