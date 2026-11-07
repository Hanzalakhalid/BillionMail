import type { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

export default {
 path: '/integrations',
 name: 'Integrations',
 component: Layout,
 redirect: '/integrations/list',
 meta: { key: 'integrations', title: 'Integrations', icon: 'i-mdi-connection', sort: 18 },
 children: [{ path: 'list', name: 'IntegrationList', component: () => import('@/views/integrations/index.vue'), meta: { title: 'Integrations' } }],
} as RouteRecordRaw
