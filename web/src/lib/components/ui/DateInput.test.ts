import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import DateInput from './DateInput.svelte';

describe('DateInput Component', () => {
	it('FT-04: renders DateInput with TEREN custom calendar icon and hidden webkit indicators', () => {
		const { container } = render(DateInput, {
			props: {
				label: 'Check In Date',
				value: '2026-05-27',
				onChange: () => {}
			}
		});

		// Check label text is rendered and has uppercase class
		const label = container.querySelector('label');
		expect(label).toBeInTheDocument();
		expect(label).toHaveTextContent('Check In Date');
		expect(label).toHaveClass('uppercase');

		// Custom TEREN SVG calendar icon must be present
		const svg = container.querySelector('svg');
		expect(svg).toBeInTheDocument();

		// Real input elements should be present
		const input = container.querySelector('input[type="date"]');
		expect(input).toBeInTheDocument();
		expect(input).toHaveValue('2026-05-27');

		// Webkit calendar picker indicator should be style-hidden (opacity-0 class)
		expect(input).toHaveClass('[&::-webkit-calendar-picker-indicator]:opacity-0');
	});
});
