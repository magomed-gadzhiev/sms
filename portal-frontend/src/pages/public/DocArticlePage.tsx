import { useParams } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { renderMarkdown, parseFrontmatter } from '../../utils/markdown';
import { DocsSidebar } from '../../components/public/DocsSidebar';

const docFiles = import.meta.glob<string>('../../content/docs/*.md', { query: '?raw', import: 'default' });

interface DocMeta { slug: string; title: string; order: number; raw: string }

export function DocArticlePage() {
  const { slug } = useParams();
  const { t } = useTranslation();
  const [docs, setDocs] = useState<DocMeta[]>([]);
  const [html, setHtml] = useState('');
  const [title, setTitle] = useState('');

  useEffect(() => {
    async function load() {
      const entries: DocMeta[] = [];
      for (const [path, loader] of Object.entries(docFiles)) {
        const raw = await loader();
        const { data } = parseFrontmatter(raw);
        const s = path.split('/').pop()!.replace('.md', '');
        entries.push({ slug: s, title: (data.title as string) || s, order: (data.order as number) || 99, raw });
      }
      entries.sort((a, b) => a.order - b.order);
      setDocs(entries);
    }
    load();
  }, []);

  useEffect(() => {
    const doc = docs.find((d) => d.slug === slug);
    if (!doc) return;
    setTitle(doc.title);
    const { content } = parseFrontmatter(doc.raw);
    renderMarkdown(content).then(setHtml);
  }, [slug, docs]);

  return (
    <>
      <Helmet>
        <title>{title ? `${title} — ${t('seo.docs.title')}` : t('seo.docs.title')}</title>
      </Helmet>
      <div className="max-w-6xl mx-auto px-4 py-12 flex flex-col md:flex-row gap-8">
        <DocsSidebar docs={docs} />
        <article
          className="flex-1 max-w-none [&_h1]:text-3xl [&_h1]:font-bold [&_h1]:mb-4 [&_h2]:text-xl [&_h2]:font-semibold [&_h2]:mt-8 [&_h2]:mb-3 [&_h3]:text-lg [&_h3]:font-semibold [&_h3]:mt-6 [&_h3]:mb-2 [&_p]:text-gray-600 [&_p]:mb-4 [&_pre]:bg-gray-50 [&_pre]:rounded-lg [&_pre]:p-4 [&_pre]:overflow-x-auto [&_code]:text-sm [&_ul]:list-disc [&_ul]:pl-6 [&_li]:text-gray-600 [&_li]:mb-1 [&_table]:w-full [&_th]:text-left [&_th]:pb-2 [&_td]:py-1 [&_td]:border-b [&_td]:border-gray-100"
          dangerouslySetInnerHTML={{ __html: html }}
        />
      </div>
    </>
  );
}
