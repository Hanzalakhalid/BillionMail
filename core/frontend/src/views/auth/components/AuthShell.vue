<template>
	<div class="auth-container">
		<section class="auth-card">
			<img class="auth-logo" src="@/assets/images/logo.png" alt="BillionMail" />
			<h1>{{ title }}</h1>
			<p v-if="description" class="auth-description">{{ description }}</p>
			<slot />
		</section>
	</div>
</template>

<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>

<style scoped>
.auth-container { min-height: 100%; display: grid; place-items: center; padding: 24px; background: var(--color-bg-3); }
.auth-card { width: min(440px, 100%); padding: 40px 32px; border-radius: 12px; background: var(--color-bg-1); box-shadow: 0 12px 40px rgb(0 0 0 / 10%); }
.auth-logo { display: block; width: 70px; margin: 0 auto 16px; }
h1 { margin: 0 0 8px; text-align: center; }
.auth-description { margin: 0 0 24px; color: var(--color-text-2); text-align: center; line-height: 1.5; }
:deep(.auth-links) { display: flex; justify-content: center; gap: 16px; margin-top: 16px; }
</style>
