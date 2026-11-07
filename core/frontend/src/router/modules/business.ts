import type { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

export default {
	path: '/business', name: 'BusinessLayout', component: Layout,
	meta: { key: 'business', title: 'Business Profile', icon: 'i-mdi-store-outline' },
	children: [{ path: '/business', name: 'BusinessProfile', component: () => import('@/views/business/index.vue'), meta: { title: 'Business Profile' } }],
} as RouteRecordRaw
