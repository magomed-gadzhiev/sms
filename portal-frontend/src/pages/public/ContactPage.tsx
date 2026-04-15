import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { Mail, MessageCircle } from 'lucide-react';
import { ContactForm } from '../../components/public/ContactForm';

export function ContactPage() {
  const { t } = useTranslation();

  return (
    <>
      <Helmet>
        <title>{t('seo.contact.title')}</title>
        <meta name="description" content={t('seo.contact.desc')} />
      </Helmet>

      <section className="bg-gradient-to-b from-indigo-50 to-white pb-12 pt-16">
        <div className="mx-auto max-w-4xl px-4 text-center">
          <h1 className="mb-3 text-4xl font-bold text-gray-900">{t('contact.title')}</h1>
          <p className="text-gray-500">{t('contact.subtitle')}</p>
        </div>
      </section>

      <section className="py-16">
        <div className="mx-auto grid max-w-5xl gap-12 px-4 md:grid-cols-2">
          <div>
            <ContactForm />
          </div>

          <div className="flex flex-col justify-center space-y-8">
            <h2 className="text-xl font-bold text-gray-900">{t('contact.directTitle')}</h2>

            <a
              href="mailto:support@smsplatform.com"
              className="flex items-center gap-4 rounded-xl border border-gray-200 p-5 transition hover:border-indigo-300 hover:shadow-sm"
            >
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-purple-500">
                <Mail size={22} className="text-white" />
              </div>
              <div>
                <p className="text-sm font-semibold text-gray-900">Email</p>
                <p className="text-sm text-gray-500">support@smsplatform.com</p>
              </div>
            </a>

            <a
              href="https://t.me/sms_support"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-4 rounded-xl border border-gray-200 p-5 transition hover:border-indigo-300 hover:shadow-sm"
            >
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500 to-purple-500">
                <MessageCircle size={22} className="text-white" />
              </div>
              <div>
                <p className="text-sm font-semibold text-gray-900">Telegram</p>
                <p className="text-sm text-gray-500">@sms_support</p>
              </div>
            </a>
          </div>
        </div>
      </section>
    </>
  );
}
