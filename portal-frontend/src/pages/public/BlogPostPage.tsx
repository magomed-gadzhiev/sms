import { useParams, Link, useLocation } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { Helmet } from 'react-helmet-async';
import { ArrowLeft } from 'lucide-react';
import { renderMarkdown, parseFrontmatter } from '../../utils/markdown';

const blogFiles = import.meta.glob<string>('../../content/blog/*.md', { query: '?raw', import: 'default' });

export function BlogPostPage() {
  const { slug } = useParams();
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';
  const [html, setHtml] = useState('');
  const [title, setTitle] = useState('');
  const [date, setDate] = useState('');

  useEffect(() => {
    async function load() {
      for (const [path, loader] of Object.entries(blogFiles)) {
        if (path.endsWith(`${slug}.md`)) {
          const raw = await loader();
          const { data, content } = parseFrontmatter(raw);
          setTitle((data.title as string) || '');
          setDate((data.date as string) || '');
          const rendered = await renderMarkdown(content);
          setHtml(rendered);
          break;
        }
      }
    }
    load();
  }, [slug]);

  return (
    <>
      <Helmet><title>{title}</title></Helmet>
      <div className="max-w-3xl mx-auto px-4 py-12">
        <Link to={`${prefix}/blog`} className="inline-flex items-center gap-1 text-sm text-indigo-600 hover:text-indigo-700 mb-6">
          <ArrowLeft size={16} /> Назад
        </Link>
        {date && <p className="text-sm text-gray-400 mb-2">{new Date(date).toLocaleDateString('ru-RU')}</p>}
        <article
          className="flex-1 max-w-none [&_h1]:text-3xl [&_h1]:font-bold [&_h1]:mb-4 [&_h2]:text-xl [&_h2]:font-semibold [&_h2]:mt-8 [&_h2]:mb-3 [&_p]:text-gray-600 [&_p]:mb-4 [&_pre]:bg-gray-50 [&_pre]:rounded-lg [&_pre]:p-4 [&_pre]:overflow-x-auto [&_code]:text-sm [&_ul]:list-disc [&_ul]:pl-6 [&_li]:text-gray-600 [&_li]:mb-1"
          dangerouslySetInnerHTML={{ __html: html }}
        />
      </div>
    </>
  );
}
