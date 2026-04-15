import { Link, useLocation } from 'react-router-dom';

interface Props { slug: string; title: string; date: string; description: string; tags: string[]; author: string }

export function BlogCard({ slug, title, date, description, tags }: Props) {
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';

  return (
    <Link to={`${prefix}/blog/${slug}`} className="block bg-white border border-gray-200 rounded-2xl p-6 hover:shadow-md transition-shadow">
      <div className="flex items-center gap-2 mb-3">
        <span className="text-sm text-gray-400">{new Date(date).toLocaleDateString('ru-RU')}</span>
        {tags.map((tag) => (
          <span key={tag} className="text-xs bg-indigo-50 text-indigo-600 px-2 py-0.5 rounded-full">{tag}</span>
        ))}
      </div>
      <h3 className="text-lg font-semibold text-gray-900 mb-2">{title}</h3>
      <p className="text-sm text-gray-500">{description}</p>
    </Link>
  );
}
