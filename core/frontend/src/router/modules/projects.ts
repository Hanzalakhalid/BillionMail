import type { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

export default {
	path: '/projects', name: 'ProjectsLayout', component: Layout,
	meta: { key: 'projects', title: 'Marketing Projects', icon: 'i-mdi-forum-outline' },
	children: [{ path: '/projects', name: 'MarketingProjects', component: () => import('@/views/projects/index.vue'), meta: { title: 'Marketing Projects' } }],
} as RouteRecordRaw
