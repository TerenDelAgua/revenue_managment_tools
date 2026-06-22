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

		// The footer button must be present. RoomDrawer renders "Check Out Guest"
		// as a hardcoded label here (not yet i18n'd — tracked as follow-up).
		const buttons = container.querySelectorAll('button');
		let primaryButton: HTMLButtonElement | null = null;
		buttons.forEach((btn) => {
			if (btn.textContent?.includes('Check Out Guest')) {
				primaryButton = btn;
			}
		});

		expect(primaryButton).toBeInTheDocument();
		// TODO(tokenize): RoomDrawer.svelte still uses bg-[#1C1917] directly.
		// Migrate the drawer to bg-teren-text-main in a follow-up PR.
		expect(primaryButton).toHaveClass('bg-[#1C1917]');
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
			// es.json → "Asignar reserva"
			if (btn.textContent?.trim() === 'Asignar reserva') {
				primaryButton = btn;
			}
		});

		expect(primaryButton).toBeInTheDocument();
		// TODO(tokenize): RoomDrawer.svelte still uses bg-[#FF8C42] directly.
		// Migrate the drawer to bg-teren-primary in a follow-up PR.
		expect(primaryButton).toHaveClass('bg-[#FF8C42]');
	});
});
