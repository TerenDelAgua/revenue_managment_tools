/**
 * ConfirmDestructive — vitest suite (Block 9 / v1.2)
 *
 * Covers the destructive confirmation modal:
 *  - CD-01 Renders nothing when closed; renders title + description when open.
 *  - CD-02 Confirm button stays disabled until the acknowledgement checkbox
 *        is ticked.
 *  - CD-03 Confirm fires onConfirm only when the box is ticked.
 *  - CD-04 Cancel button fires onCancel.
 *  - CD-05 Backdrop click fires onCancel.
 *  - CD-06 Escape key fires onCancel.
 *  - CD-07 Re-opening resets the acknowledgement (must re-tick).
 */
import { describe, it, expect, beforeAll, beforeEach, vi, afterEach } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import { locale } from 'svelte-i18n';
import ConfirmDestructive from './ConfirmDestructive.svelte';

const baseProps = {
	open: true,
	title: 'Change refund method?',
	description:
		'The refund method will differ from the original payment method. This change is recorded in the audit trail.',
	checkboxLabel: 'I understand this changes the audit trail',
	confirmLabel: 'Confirm change',
	cancelLabel: 'Keep original',
	onConfirm: vi.fn(),
	onCancel: vi.fn()
};

beforeAll(() => {
	locale.set('en');
});

beforeEach(() => {
	baseProps.onConfirm.mockClear();
	baseProps.onCancel.mockClear();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('ConfirmDestructive', () => {
	it('CD-01: renders nothing when closed', () => {
		const { queryByTestId } = render(ConfirmDestructive, {
			props: { ...baseProps, open: false }
		});
		expect(queryByTestId('confirm-destructive-dialog')).toBeNull();
	});

	it('CD-01b: renders title + description + checkbox when open', () => {
		const { getByTestId } = render(ConfirmDestructive, { props: baseProps });
		const dialog = getByTestId('confirm-destructive-dialog');
		expect(dialog).toHaveTextContent('Change refund method?');
		expect(dialog).toHaveTextContent(/audit trail/i);
		expect(getByTestId('confirm-destructive-checkbox')).toBeInTheDocument();
		expect(getByTestId('confirm-destructive-confirm')).toHaveTextContent('Confirm change');
		expect(getByTestId('confirm-destructive-cancel')).toHaveTextContent('Keep original');
	});

	it('CD-02: confirm button is disabled until the checkbox is ticked', async () => {
		const { getByTestId } = render(ConfirmDestructive, { props: baseProps });
		const confirm = getByTestId('confirm-destructive-confirm') as HTMLButtonElement;
		expect(confirm.disabled).toBe(true);

		await fireEvent.click(getByTestId('confirm-destructive-checkbox'));
		expect(confirm.disabled).toBe(false);
	});

	it('CD-03: confirm fires onConfirm only when acknowledged', async () => {
		const { getByTestId } = render(ConfirmDestructive, { props: baseProps });
		const confirm = getByTestId('confirm-destructive-confirm') as HTMLButtonElement;

		// Disabled → click is a no-op.
		await fireEvent.click(confirm);
		expect(baseProps.onConfirm).not.toHaveBeenCalled();

		// Tick → click now fires.
		await fireEvent.click(getByTestId('confirm-destructive-checkbox'));
		await fireEvent.click(confirm);
		expect(baseProps.onConfirm).toHaveBeenCalledTimes(1);
	});

	it('CD-04: cancel button fires onCancel', async () => {
		const { getByTestId } = render(ConfirmDestructive, { props: baseProps });
		await fireEvent.click(getByTestId('confirm-destructive-cancel'));
		expect(baseProps.onCancel).toHaveBeenCalledTimes(1);
	});

	it('CD-05: backdrop click fires onCancel', async () => {
		const { getByTestId } = render(ConfirmDestructive, { props: baseProps });
		// Clicking on the backdrop element itself triggers onCancel;
		// clicks on the dialog content bubble up but the handler checks
		// `e.target === e.currentTarget`.
		await fireEvent.click(getByTestId('confirm-destructive-backdrop'));
		expect(baseProps.onCancel).toHaveBeenCalledTimes(1);
	});

	it('CD-05b: click on dialog content does NOT fire onCancel', async () => {
		const { getByTestId } = render(ConfirmDestructive, { props: baseProps });
		await fireEvent.click(getByTestId('confirm-destructive-dialog'));
		expect(baseProps.onCancel).not.toHaveBeenCalled();
	});

	it('CD-06: Escape key fires onCancel', async () => {
		const { getByTestId } = render(ConfirmDestructive, { props: baseProps });
		await fireEvent.keyDown(getByTestId('confirm-destructive-backdrop'), { key: 'Escape' });
		expect(baseProps.onCancel).toHaveBeenCalledTimes(1);
	});

	it('CD-07: re-opening resets the checkbox (must re-acknowledge)', async () => {
		const { rerender, getByTestId } = render(ConfirmDestructive, { props: baseProps });
		// Tick → confirm becomes enabled.
		await fireEvent.click(getByTestId('confirm-destructive-checkbox'));
		expect(
			(getByTestId('confirm-destructive-confirm') as HTMLButtonElement).disabled
		).toBe(false);

		// Close + re-open with `open` toggle — the modal unmounts and the
		// checkbox state is recreated from $state(false) on the next mount.
		await rerender({ ...baseProps, open: false });
		await rerender({ ...baseProps, open: true });

		const confirm = getByTestId('confirm-destructive-confirm') as HTMLButtonElement;
		expect(confirm.disabled).toBe(true);
	});

	it('CD-08: dialog has the right accessibility attributes', () => {
		const { getByTestId } = render(ConfirmDestructive, { props: baseProps });
		const dialog = getByTestId('confirm-destructive-dialog');
		expect(dialog.getAttribute('role')).toBe('dialog');
		expect(dialog.getAttribute('aria-modal')).toBe('true');
		expect(dialog.getAttribute('aria-labelledby')).toBe('confirm-destructive-title');
		expect(dialog.getAttribute('aria-describedby')).toBe('confirm-destructive-description');
	});
});