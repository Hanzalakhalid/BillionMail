import type { SelectOption } from 'naive-ui'
import { instance } from '@/api'
import { isArray, isObject } from '@/utils'
import { getGroupAll } from '@/api/modules/contacts/group'
import { getTemplateAll } from '@/api/modules/market/template'
import { getTaskList } from '@/api/modules/market/task'
import {
	getWhatsappConfig,
	getWhatsappTemplates,
	type WhatsappConfig,
	type WhatsappTemplate,
} from '@/api/modules/whatsapp'
import type { Lookups } from './presets'

/** Groups, tags, templates, campaigns and WhatsApp templates used by the workflow pages. */
export const useWorkflowLookups = () => {
	const groupOptions = ref<SelectOption[]>([])
	const tagOptions = ref<SelectOption[]>([])
	const templateOptions = ref<SelectOption[]>([])
	const campaignOptions = ref<SelectOption[]>([])
	const waTemplates = ref<WhatsappTemplate[]>([])

	const load = async () => {
		const [groups, tags, templates, tasks] = await Promise.allSettled([
			getGroupAll(),
			instance.get('/tags/all'),
			getTemplateAll(),
			getTaskList({ page: 1, page_size: 200, keyword: '' }),
		])
		if (groups.status === 'fulfilled' && isObject<{ list: { id: number; name: string }[] | null }>(groups.value)) {
			groupOptions.value = (groups.value.list ?? []).map(g => ({ label: g.name, value: g.id }))
		}
		if (
			tags.status === 'fulfilled' &&
			isObject<{ list: { id: number; name: string; group_name: string }[] | null }>(tags.value)
		) {
			tagOptions.value = (tags.value.list ?? []).map(t => ({
				label: t.group_name ? `${t.name} (${t.group_name})` : t.name,
				value: t.id,
			}))
		}
		if (
			tasks.status === 'fulfilled' &&
			isObject<{ list: { id: number; subject: string; task_name?: string }[] | null }>(tasks.value)
		) {
			campaignOptions.value = (tasks.value.list ?? []).map(t => ({
				label: `${t.subject || t.task_name || 'Campaign'} (#${t.id})`,
				value: t.id,
			}))
		}
		if (templates.status === 'fulfilled' && isArray<{ id: number; temp_name: string }>(templates.value)) {
			templateOptions.value = templates.value.map(t => ({ label: t.temp_name, value: t.id }))
		}
		try {
			// Only ask Meta for templates when WhatsApp is set up, to avoid an error toast
			const cfg = await getWhatsappConfig()
			if (!isObject<WhatsappConfig>(cfg) || !cfg.business_account_id || !cfg.access_token) return
			const res = await getWhatsappTemplates()
			waTemplates.value = isArray<WhatsappTemplate>(res) ? res.filter(t => t.status === 'APPROVED') : []
		} catch {
			waTemplates.value = []
		}
	}

	const labelOf = (options: SelectOption[], id: number) =>
		String(options.find(o => o.value === id)?.label ?? `#${id}`)

	const lookups: Lookups = {
		groupName: id => labelOf(groupOptions.value, id),
		tagName: id => labelOf(tagOptions.value, id),
		templateName: id => labelOf(templateOptions.value, id),
		campaignName: id => labelOf(campaignOptions.value, id),
	}

	return { groupOptions, tagOptions, templateOptions, campaignOptions, waTemplates, lookups, load }
}
