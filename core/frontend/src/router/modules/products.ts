import type { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

export default {
	path: '/products', name: 'ProductsLayout', component: Layout,
	meta: { key: 'products', title: 'Products', icon: 'i-mdi-package-variant-closed' },
	children: [{ path: '/products', name: 'Products', component: () => import('@/views/products/index.vue'), meta: { title: 'Products' } }],
} as RouteRecordRaw
