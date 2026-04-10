import { useNavigate } from 'react-router-dom';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';

export function SmppSettingsPage() {
  const navigate = useNavigate();
  return (
    <div>
      <PageHeader
        title="Настройки SMPP"
        subtitle="Управление SMPP-подключениями для отправки сообщений"
      />
      <div className="bg-white rounded-lg border p-6 max-w-lg">
        <p className="text-gray-600 mb-4">
          Настройка SMPP-подключений позволяет отправлять SMS через ваш собственный провайдер.
        </p>
        <Button onClick={() => navigate('/providers')}>
          Управление SMPP-провайдерами
        </Button>
      </div>
    </div>
  );
}
