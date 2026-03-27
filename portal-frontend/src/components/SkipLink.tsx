interface SkipLinkProps {
  targetId: string;
}

export function SkipLink({ targetId }: SkipLinkProps) {
  return (
    <a
      href={`#${targetId}`}
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        padding: '16px 24px',
        background: '#1976d2',
        color: '#fff',
        borderRadius: '0 0 8px 0',
        textDecoration: 'none',
        fontWeight: 'bold',
        zIndex: 999999,
        transform: 'translateY(-100%)',
        transition: 'transform 0.2s ease-in-out',
      }}
      onFocus={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.transform = 'translateY(0)';
      }}
      onBlur={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.transform = 'translateY(-100%)';
      }}
    >
      Skip to main content
    </a>
  );
}
