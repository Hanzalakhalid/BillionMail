import { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

const route: RouteRecordRaw = {
	path: '/ad-finder',
	name: 'AdFinderLayout',
	component: Layout,
	meta: {
		sort: 7,
		key: 'ad-finder',
		title: 'Ad Finder',
		titleKey: 'layout.menu.adFinder',
	},
	children: [
		{
			path: '/ad-finder',
			name: 'AdFinder',
			meta: { title: 'Ad Finder', titleKey: 'layout.menu.adFinder' },
			component: () => import('@/views/ad-finder/index.vue'),
		},
	],
}

export default route
