interface SkipLinkProps {
  targetId: string;
}

export function SkipLink({ targetId }: SkipLinkProps) {
  return (
    <a
      href={`#${targetId}`}
      style={{
        position: 'absolute',
        top: 8,
        left: 8,
        padding: '8px 16px',
        background: '#1976d2',
        color: '#fff',
        borderRadius: 4,
        textDecoration: 'none',
        fontWeight: 'bold',
        zIndex: 9999,
        transform: 'translateY(-100%)',
        transition: 'transform 0.1s',
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
