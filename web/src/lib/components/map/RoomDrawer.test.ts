import { describe, it, expect, beforeAll } from 'vitest';
import { render } from '@testing-library/svelte';
import { locale } from 'svelte-i18n';
import RoomDrawer from './RoomDrawer.svelte';
import type { RoomMap } from '$lib/types';

describe('RoomDrawer Component', () => {
	// The component is asserted against Spanish translations (status + button
	// labels), matching the es.json locale. The setupTests.ts registers all
	// locales; here we just activate the one this suite targets.
	beforeAll(() => {
		locale.set('es');
	});

	const mockRoom = (availability: RoomMap['availability']): RoomMap => ({
		id: 'test-room-uuid-long-enough',
		number: '101',
		pos_x: 0,
		pos_y: 0,
		room_type: { id: 'type-id', name: 'Standard Suite' },
		availability,
		has_bookings: false,
		active_booking: null,
		pending_booking: null,
		block: null
	});

	it('FT-05: opens drawer for occupied room and renders "Check-out" primary action button', () => {
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

		// Render details header & status (es locale)
		expect(container).toHaveTextContent('101');
		expect(container).toHaveTextContent('Standard Suite · Ocupada');

		// The footer button must be present and display "Check-out" (es)
		const buttons = container.querySelectorAll('button');
		let primaryButton: HTMLButtonElement | null = null;
		buttons.forEach((btn) => {
			if (btn.textContent?.includes('Check-out')) {
				primaryButton = btn;
			}
		});

		expect(primaryButton).toBeInTheDocument();
		expect(primaryButton).toHaveClass('bg-teren-text-main');
	});

	it('FT-06: opens drawer for available room and renders "Asignar reserva" primary action button', () => {
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
			// es.json → "Asignar reserva"
			if (btn.textContent?.includes('Asignar reserva')) {
				primaryButton = btn;
			}
		});

		expect(primaryButton).toBeInTheDocument();
		expect(primaryButton).toHaveClass('bg-teren-primary');
	});
});
