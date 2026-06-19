import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import RoomToken from './RoomToken.svelte';
import type { RoomMap } from '$lib/types';

describe('RoomToken Component', () => {
	const mockRoom = (availability: RoomMap['availability']): RoomMap => ({
		id: 'test-room-id',
		number: '101',
		pos_x: 0,
		pos_y: 0,
		room_type: { id: 'type-id', name: 'Standard' },
		availability,
		active_booking: null,
		pending_booking: null,
		block: null
	});

	it('FT-01: renders available room token with green bg, text and clean state', () => {
		const room = mockRoom('available');
		const { container } = render(RoomToken, { props: { room, mode: 'ops', onSelect: () => {} } });

		const token = container.querySelector('.room-token');
		expect(token).toBeInTheDocument();
		expect(token).toHaveClass('bg-[#16A34A]');
		expect(token).toHaveTextContent('101');
		// Ensure no icons are displayed for available state
		expect(token).not.toHaveTextContent('🛏️');
		expect(token).not.toHaveTextContent('⏳');
		expect(token).not.toHaveTextContent('🔧');
	});

	it('FT-02: renders occupied room token with red bg and bed icon', () => {
		const room = mockRoom('occupied');
		const { container } = render(RoomToken, { props: { room, mode: 'ops', onSelect: () => {} } });

		const token = container.querySelector('.room-token');
		expect(token).toBeInTheDocument();
		expect(token).toHaveClass('bg-[#DC2626]');
		expect(token).toHaveTextContent('🛏️');
	});

	it('FT-03: renders blocked room token with dark bg, striped pattern class and wrench icon', () => {
		const room = mockRoom('blocked');
		const { container } = render(RoomToken, { props: { room, mode: 'ops', onSelect: () => {} } });

		const token = container.querySelector('.room-token');
		expect(token).toBeInTheDocument();
		expect(token).toHaveClass('bg-[#44403C]');
		expect(token).toHaveTextContent('🔧');
		// Striped linear gradient pattern class check
		expect(token).toHaveClass('bg-[repeating-linear-gradient(45deg,transparent,transparent_4px,rgba(255,255,255,0.1)_4px,rgba(255,255,255,0.1)_8px)]');
	});

	it('FT-04: renders cleaning room token with sky-600 bg and broom icon', () => {
		const room = mockRoom('cleaning');
		const { container } = render(RoomToken, { props: { room, mode: 'ops', onSelect: () => {} } });

		const token = container.querySelector('.room-token');
		expect(token).toBeInTheDocument();
		expect(token).toHaveClass('bg-[#0284C7]');
		expect(token).toHaveTextContent('🧹');
		// Cleaning rooms are NOT available: must not show green or any "ready" affordance
		expect(token).not.toHaveClass('bg-[#16A34A]');
		// No striped pattern (cleaning is a solid operational state, not a block)
		expect(token).not.toHaveClass('bg-[repeating-linear-gradient(45deg,transparent,transparent_4px,rgba(255,255,255,0.1)_4px,rgba(255,255,255,0.1)_8px)]');
	});
});
