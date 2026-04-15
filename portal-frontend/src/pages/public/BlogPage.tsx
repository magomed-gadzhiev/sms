import { useEffect, useState } from 'react';
import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { parseFrontmatter } from '../../utils/markdown';
import { BlogCard } from '../../components/public/BlogCard';

const blogFiles = import.meta.glob<string>('../../content/blog/*.md', { query: '?raw', import: 'default' });

interface BlogMeta { slug: string; title: string; date: string; description: string; tags: string[]; author: string }

export function BlogPage() {
  const { t } = useTranslation();
  const [posts, setPosts] = useState<BlogMeta[]>([]);

  useEffect(() => {
    async function load() {
      const entries: BlogMeta[] = [];
      for (const [path, loader] of Object.entries(blogFiles)) {
        const raw = await loader();
        const { data } = parseFrontmatter(raw);
        const slug = path.split('/').pop()!.replace('.md', '');
        entries.push({
          slug,
          title: (data.title as string) || slug,
          date: (data.date as string) || '',
          description: (data.description as string) || '',
          tags: (data.tags as string[]) || [],
          author: (data.author as string) || '',
        });
      }
      entries.sort((a, b) => b.date.localeCompare(a.date));
      setPosts(entries);
    }
    load();
  }, []);

  return (
    <>
      <Helmet>
        <title>{t('seo.blog.title')}</title>
        <meta name="description" content={t('seo.blog.desc')} />
      </Helmet>
      <section className="pt-16 pb-8 bg-gradient-to-b from-indigo-50 to-white">
        <div className="max-w-4xl mx-auto px-4 text-center">
          <h1 className="text-4xl font-bold text-gray-900">{t('nav.blog')}</h1>
        </div>
      </section>
      <section className="py-12">
        <div className="max-w-4xl mx-auto px-4 space-y-4">
          {posts.map((post) => <BlogCard key={post.slug} {...post} />)}
        </div>
      </section>
    </>
  );
}
