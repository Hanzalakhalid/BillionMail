import type { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

export default {
	path: '/assets',
	name: 'AssetsLayout',
	component: Layout,
	meta: { key: 'assets', title: 'Asset Library', icon: 'i-mdi-image-multiple', sort: 18 },
	children: [{ path: '/assets', name: 'Assets', component: () => import('@/views/assets/index.vue'), meta: { title: 'Asset Library' } }],
} as RouteRecordRaw
