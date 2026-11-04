import { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

const route: RouteRecordRaw = {
	path: '/team',
	name: 'TeamLayout',
	component: Layout,
	meta: { key: 'team', title: 'Team Members', titleKey: 'layout.menu.team' },
	children: [{
		path: '/team',
		name: 'Team',
		meta: { title: 'Team Members', titleKey: 'layout.menu.team' },
		component: () => import('@/views/team/index.vue'),
	}],
}

export default route
