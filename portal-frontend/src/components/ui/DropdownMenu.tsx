import * as RadixDropdown from '@radix-ui/react-dropdown-menu';
import type { ReactNode } from 'react';

export type DropdownMenuItem =
  | { label: string; onSelect: () => void; variant?: 'default' | 'danger'; disabled?: boolean }
  | { separator: true };

interface DropdownMenuProps {
  trigger: ReactNode;
  items: DropdownMenuItem[];
  align?: 'left' | 'right';
}

export function DropdownMenu({ trigger, items, align = 'right' }: DropdownMenuProps) {
  return (
    <RadixDropdown.Root>
      <RadixDropdown.Trigger asChild>{trigger}</RadixDropdown.Trigger>
      <RadixDropdown.Portal>
        <RadixDropdown.Content
          align={align === 'right' ? 'end' : 'start'}
          sideOffset={4}
          className="z-50 min-w-[180px] bg-white dark:bg-slate-800
                     border border-gray-200 dark:border-slate-700 rounded-md shadow-lg py-1"
        >
          {items.map((item, idx) => {
            if ('separator' in item) {
              return (
                <RadixDropdown.Separator
                  key={idx}
                  className="my-1 border-t border-gray-100 dark:border-slate-700 pointer-events-none"
                />
              );
            }
            const isDanger = item.variant === 'danger';
            return (
              <RadixDropdown.Item
                key={idx}
                disabled={item.disabled}
                onSelect={item.onSelect}
                className={`w-full text-left px-3 py-2 text-sm cursor-pointer outline-none
                            data-[disabled]:opacity-50 data-[disabled]:cursor-not-allowed
                            data-[highlighted]:bg-gray-100 dark:data-[highlighted]:bg-slate-700
                            ${isDanger
                              ? 'text-red-600 dark:text-red-400 data-[highlighted]:bg-red-50 dark:data-[highlighted]:bg-red-950/30'
                              : 'text-gray-700 dark:text-slate-200'}`}
              >
                {item.label}
              </RadixDropdown.Item>
            );
          })}
        </RadixDropdown.Content>
      </RadixDropdown.Portal>
    </RadixDropdown.Root>
  );
}
