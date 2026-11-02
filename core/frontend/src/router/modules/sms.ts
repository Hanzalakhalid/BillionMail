import { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

const route: RouteRecordRaw = {
	path: '/sms',
	name: 'SmsLayout',
	component: Layout,
	meta: {
		sort: 7,
		key: 'sms',
		title: 'SMS',
		titleKey: 'layout.menu.sms',
	},
	children: [
		{
			path: '/sms',
			name: 'Sms',
			meta: { title: 'SMS', titleKey: 'layout.menu.sms' },
			component: () => import('@/views/sms/index.vue'),
		},
	],
}

export default route
