import { type ButtonHTMLAttributes, type ReactNode, forwardRef } from 'react';

const variantStyles = {
  primary: 'bg-primary text-white hover:bg-primary-dark',
  secondary: 'bg-gray-100 text-gray-800 hover:bg-gray-200 border border-gray-300 dark:bg-slate-800 dark:text-slate-100 dark:hover:bg-slate-700 dark:border-slate-700',
  danger: 'bg-danger text-white hover:bg-red-800',
  ghost: 'text-gray-600 hover:bg-gray-100 dark:text-slate-300 dark:hover:bg-slate-800',
} as const;

const sizeStyles = {
  sm: 'px-3 py-2 text-xs min-h-9',
  md: 'px-4 py-2 text-sm min-h-10',
  lg: 'px-5 py-2.5 text-base min-h-11',
} as const;

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: keyof typeof variantStyles;
  size?: keyof typeof sizeStyles;
  children: ReactNode;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', className = '', children, ...props }, ref) => (
    <button
      ref={ref}
      className={`inline-flex items-center justify-center gap-2 rounded font-medium transition-colors
        focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50
        active:scale-[0.97]
        disabled:opacity-50 disabled:cursor-not-allowed
        ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  ),
);

Button.displayName = 'Button';
