import { useCallback, useEffect, useRef, useState } from 'react';
import { searchApi, type CommandItem } from '../api/client';

export function useCommandPalette() {
  const [isOpen, setIsOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<CommandItem[]>([]);
  const [activeIndex, setActiveIndex] = useState(0);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const open = useCallback(() => {
    setIsOpen(true);
    setQuery('');
    setResults([]);
    setActiveIndex(0);
  }, []);

  const close = useCallback(() => {
    setIsOpen(false);
    setQuery('');
    setResults([]);
  }, []);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    if (query.length < 2) {
      setResults([]);
      return;
    }
    debounceRef.current = setTimeout(async () => {
      try {
        const res = await searchApi.search(query);
        setResults(res.items ?? []);
        setActiveIndex(0);
      } catch {
        setResults([]);
      }
    }, 300);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query]);

  const navigate = useCallback((direction: 'up' | 'down', totalItems: number) => {
    setActiveIndex((prev) => {
      if (direction === 'down') return (prev + 1) % totalItems;
      return (prev - 1 + totalItems) % totalItems;
    });
  }, []);

  return { isOpen, query, setQuery, results, activeIndex, open, close, navigate };
}
