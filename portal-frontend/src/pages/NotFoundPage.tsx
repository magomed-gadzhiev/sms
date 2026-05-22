import { Link } from 'react-router-dom';

export function NotFoundPage() {
  return (
    <div className="min-h-[60vh] flex items-center justify-center px-4">
      <div className="text-center max-w-md">
        <p className="text-6xl font-bold text-gray-300">404</p>
        <h1 className="mt-4 text-xl font-semibold text-gray-800">Страница не найдена</h1>
        <p className="mt-2 text-sm text-gray-500">
          Адрес введён неверно или раздел был перенесён.
        </p>
        <Link
          to="/command-center"
          className="mt-6 inline-block px-4 py-2 rounded-md bg-primary text-white text-sm font-medium hover:bg-primary/90"
        >
          На главную
        </Link>
      </div>
    </div>
  );
}
