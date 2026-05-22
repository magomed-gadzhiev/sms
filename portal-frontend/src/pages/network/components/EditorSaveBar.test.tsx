import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { EditorSaveBar } from './EditorSaveBar';

const baseProps = {
  saving: false,
  onSave: vi.fn(),
  onCancel: vi.fn(),
};

describe('EditorSaveBar', () => {
  describe('pluralization', () => {
    it('renders "1 изменение"', () => {
      render(<EditorSaveBar {...baseProps} dirtyCount={1} />);
      expect(screen.getByText(/изменение/)).toBeInTheDocument();
      expect(screen.getByText(/1/)).toBeInTheDocument();
    });

    it('renders "2 изменения"', () => {
      render(<EditorSaveBar {...baseProps} dirtyCount={2} />);
      expect(screen.getByText(/изменения/)).toBeInTheDocument();
    });

    it('renders "5 изменений"', () => {
      render(<EditorSaveBar {...baseProps} dirtyCount={5} />);
      expect(screen.getByText(/изменений/)).toBeInTheDocument();
    });

    it('renders "11 изменений" (teen edge case)', () => {
      render(<EditorSaveBar {...baseProps} dirtyCount={11} />);
      expect(screen.getByText(/изменений/)).toBeInTheDocument();
    });

    it('renders "21 изменение" (mod10=1, mod100!=11)', () => {
      render(<EditorSaveBar {...baseProps} dirtyCount={21} />);
      expect(screen.getByText(/изменение/)).toBeInTheDocument();
    });
  });

  describe('visibility', () => {
    it('has aria-hidden="true" when dirtyCount=0', () => {
      const { container } = render(<EditorSaveBar {...baseProps} dirtyCount={0} />);
      const region = container.firstChild as HTMLElement;
      expect(region).toHaveAttribute('aria-hidden', 'true');
    });

    it('does not have aria-hidden when dirtyCount>0', () => {
      const { container } = render(<EditorSaveBar {...baseProps} dirtyCount={3} />);
      const region = container.firstChild as HTMLElement;
      expect(region).toHaveAttribute('aria-hidden', 'false');
    });
  });

  describe('interactions', () => {
    it('calls onSave when «Сохранить» is clicked', async () => {
      const onSave = vi.fn();
      render(<EditorSaveBar {...baseProps} dirtyCount={1} onSave={onSave} />);
      await userEvent.click(screen.getByRole('button', { name: /сохранить/i }));
      expect(onSave).toHaveBeenCalledOnce();
    });

    it('calls onCancel when «Отменить» is clicked', async () => {
      const onCancel = vi.fn();
      render(<EditorSaveBar {...baseProps} dirtyCount={1} onCancel={onCancel} />);
      await userEvent.click(screen.getByRole('button', { name: /отменить/i }));
      expect(onCancel).toHaveBeenCalledOnce();
    });

    it('shows "Сохранение…" and disables buttons while saving', () => {
      render(<EditorSaveBar {...baseProps} dirtyCount={2} saving={true} />);
      expect(screen.getByText('Сохранение…')).toBeInTheDocument();
      const buttons = screen.getAllByRole('button');
      buttons.forEach((btn) => {
        expect(btn).toBeDisabled();
      });
    });
  });
});
