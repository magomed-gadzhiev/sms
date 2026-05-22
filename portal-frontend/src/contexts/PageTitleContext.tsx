import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';

interface PageTitleState {
  title: string;
  setTitle: (t: string) => void;
}

const PageTitleContext = createContext<PageTitleState | null>(null);

export function PageTitleProvider({ children }: { children: ReactNode }) {
  const [title, setTitle] = useState('');
  return (
    <PageTitleContext.Provider value={{ title, setTitle }}>
      {children}
    </PageTitleContext.Provider>
  );
}

/**
 * usePageTitle — sets the page title in the global AppHeader.
 * Clears on unmount so the next page either sets its own title or falls
 * back to the route-name map in AppHeader.
 */
export function usePageTitle(title: string) {
  const ctx = useContext(PageTitleContext);
  const setTitle = ctx?.setTitle;
  useEffect(() => {
    if (!setTitle) return;
    setTitle(title);
    return () => setTitle('');
  }, [title, setTitle]);
}

export function usePageTitleValue(): string {
  const ctx = useContext(PageTitleContext);
  return ctx?.title ?? '';
}
