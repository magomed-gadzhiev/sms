import { Link, useParams, useLocation } from 'react-router-dom';

interface DocEntry { slug: string; title: string }

export function DocsSidebar({ docs }: { docs: DocEntry[] }) {
  const { slug } = useParams();
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';

  return (
    <nav className="w-full md:w-56 shrink-0">
      <ul className="space-y-1">
        {docs.map((doc) => (
          <li key={doc.slug}>
            <Link
              to={`${prefix}/docs/${doc.slug}`}
              className={`block px-3 py-2 rounded-lg text-sm transition-colors ${
                slug === doc.slug
                  ? 'bg-indigo-50 text-indigo-700 font-medium'
                  : 'text-gray-600 hover:bg-gray-50'
              }`}
            >
              {doc.title}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}
