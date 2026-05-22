import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DropdownMenu } from './DropdownMenu';

describe('DropdownMenu', () => {
  const items = [
    { label: 'Edit', onSelect: () => {} },
    { label: 'Delete', onSelect: () => {}, variant: 'danger' as const },
  ];

  it('renders trigger and hides menu by default', () => {
    render(<DropdownMenu trigger={<button>menu</button>} items={items} />);
    expect(screen.getByRole('button', { name: 'menu' })).toBeInTheDocument();
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('opens menu on trigger click', async () => {
    const user = userEvent.setup();
    render(<DropdownMenu trigger={<button>menu</button>} items={items} />);
    await user.click(screen.getByRole('button', { name: 'menu' }));
    expect(await screen.findByRole('menu')).toBeInTheDocument();
    expect(screen.getAllByRole('menuitem')).toHaveLength(2);
  });

  it('calls onSelect and closes menu', async () => {
    const user = userEvent.setup();
    let called = false;
    const customItems = [{ label: 'Edit', onSelect: () => { called = true; } }];
    render(<DropdownMenu trigger={<button>menu</button>} items={customItems} />);
    await user.click(screen.getByRole('button', { name: 'menu' }));
    await user.click(await screen.findByRole('menuitem', { name: 'Edit' }));
    expect(called).toBe(true);
    // Radix unmounts content on close — menu should disappear
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('closes on Escape', async () => {
    const user = userEvent.setup();
    render(<DropdownMenu trigger={<button>menu</button>} items={items} />);
    await user.click(screen.getByRole('button', { name: 'menu' }));
    await screen.findByRole('menu');
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('renders separator between items', async () => {
    const user = userEvent.setup();
    const itemsWithSep = [
      { label: 'Edit', onSelect: () => {} },
      { separator: true as const },
      { label: 'Delete', onSelect: () => {}, variant: 'danger' as const },
    ];
    render(<DropdownMenu trigger={<button>menu</button>} items={itemsWithSep} />);
    await user.click(screen.getByRole('button', { name: 'menu' }));
    expect(await screen.findByRole('separator')).toBeInTheDocument();
  });
});
