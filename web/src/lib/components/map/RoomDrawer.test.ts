import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import RoomDrawer from './RoomDrawer.svelte';
import type { RoomMap } from '$lib/types';

describe('RoomDrawer Component', () => {
	const mockRoom = (availability: RoomMap['availability']): RoomMap => ({
		id: 'test-room-uuid-long-enough',
		number: '101',
		pos_x: 0,
		pos_y: 0,
		room_type: { id: 'type-id', name: 'Standard Suite' },
		availability,
		active_booking: null,
		pending_booking: null,
		block: null
	});

	it('FT-05: opens drawer for occupied room and renders "Check Out" primary action button', () => {
		const room = mockRoom('occupied');
		const { container } = render(RoomDrawer, {
			props: {
				room,
				propertyId: 'prop-id',
				isOpen: true,
				onClose: () => {},
				onAction: () => {}
			}
		});

		// Render details header & status
		expect(container).toHaveTextContent('101');
		expect(container).toHaveTextContent('Standard Suite · Ocupada');

		// The footer button must be present and display "Check Out"
		const buttons = container.querySelectorAll('button');
		let primaryButton: HTMLButtonElement | null = null;
		buttons.forEach((btn) => {
			if (btn.textContent?.trim() === 'Check Out') {
				primaryButton = btn;
			}
		});

		expect(primaryButton).toBeInTheDocument();
		expect(primaryButton).toHaveClass('bg-[#1C1917]'); // Spec styling color for checkout
	});

	it('FT-06: opens drawer for available room and renders "Assign Booking" primary action button', () => {
		const room = mockRoom('available');
		const { container } = render(RoomDrawer, {
			props: {
				room,
				propertyId: 'prop-id',
				isOpen: true,
				onClose: () => {},
				onAction: () => {}
			}
		});

		expect(container).toHaveTextContent('Standard Suite · Disponible');

		const buttons = container.querySelectorAll('button');
		let primaryButton: HTMLButtonElement | null = null;
		buttons.forEach((btn) => {
			if (btn.textContent?.trim() === 'Assign Booking') {
				primaryButton = btn;
			}
		});

		expect(primaryButton).toBeInTheDocument();
		expect(primaryButton).toHaveClass('bg-[#FF8C42]'); // Spec styling color for assign
	});
});
