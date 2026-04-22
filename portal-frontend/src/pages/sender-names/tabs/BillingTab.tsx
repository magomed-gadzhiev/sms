import type { SenderNameInfo } from '../../../api/client';

export function BillingTab({ senderName }: { senderName: SenderNameInfo }) {
  return (
    <div className="text-center py-12 text-gray-500">
      <p className="mb-1">Раздел в разработке</p>
      <p className="text-sm text-gray-400">Sender name: {senderName.name}</p>
    </div>
  );
}
