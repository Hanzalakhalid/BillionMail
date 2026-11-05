import { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

const route: RouteRecordRaw = {
	path: '/automation',
	name: 'AutomationLayout',
	component: Layout,
	meta: {
		sort: 3,
		key: 'automation',
		title: 'Workflows',
		titleKey: 'layout.menu.workflows',
		icon: 'i-mdi-refresh-auto',
	},
	children: [
		{
			path: '/automation',
			name: 'Automation',
			meta: { title: 'Workflows', titleKey: 'layout.menu.workflows' },
			component: () => import('@/views/automation/index.vue'),
		},
		{
			path: '/automation/edit',
			name: 'AutomationEdit',
			meta: { title: 'Workflow', titleKey: 'layout.menu.workflows', hidden: true },
			component: () => import('@/views/automation/edit.vue'),
		},
	],
}

export default route
