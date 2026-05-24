/** @type {import('tailwindcss').Config} */
export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	theme: {
		extend: {
			colors: {
				teren: {
					'background-base': '#F5F4F1',
					'surface-base': '#FCFBFA',
					primary: '#FF8C42',
					'primary-hover': '#E06B20',
					'primary-subtle': '#FFF7ED',
					'text-main': '#1C1917',
					'text-muted': '#57534E',
					'border-subtle': '#E7E5E4'
				}
			},
			fontFamily: {
				sans: ['Inter', 'Segoe UI', 'Tahoma', 'Geneva', 'Verdana', 'sans-serif']
			}
		}
	},
	plugins: []
};
