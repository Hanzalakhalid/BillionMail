import { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

const route: RouteRecordRaw = {
	path: '/push',
	name: 'PushLayout',
	component: Layout,
	meta: {
		sort: 7,
		key: 'push',
		title: 'Push',
		titleKey: 'layout.menu.push',
	},
	children: [
		{
			path: '/push',
			name: 'Push',
			meta: { title: 'Push', titleKey: 'layout.menu.push' },
			component: () => import('@/views/push/index.vue'),
		},
	],
}

export default route
