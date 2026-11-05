import { RouteRecordRaw } from 'vue-router'
import { Layout } from '@/router/constant'

const route: RouteRecordRaw = {
	path: '/whatsapp',
	name: 'WhatsappLayout',
	component: Layout,
	meta: {
		sort: 7,
		key: 'whatsapp',
		title: 'WhatsApp',
		titleKey: 'layout.menu.whatsapp',
	},
	children: [
		{
			path: '/whatsapp',
			name: 'Whatsapp',
			meta: { title: 'WhatsApp', titleKey: 'layout.menu.whatsapp' },
			component: () => import('@/views/whatsapp/index.vue'),
		},
		{
			path: '/whatsapp/bots',
			name: 'WhatsappBots',
			meta: { title: 'Chatbots', icon: 'i-mdi-robot-happy-outline' },
			component: () => import('@/views/whatsapp/bots/index.vue'),
		},
		{
			path: '/whatsapp/bots/edit',
			name: 'WhatsappBotEdit',
			meta: { title: 'Bot Builder', hidden: true },
			component: () => import('@/views/whatsapp/bots/edit.vue'),
		},
	],
}

export default route
