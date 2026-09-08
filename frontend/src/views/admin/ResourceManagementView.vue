<template>
  <OperatorResources v-if="auth.isOperator" :key="resource" :resource="resource" />
  <component :is="adminView" v-else-if="auth.isAdmin" />
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { useAuthStore } from '@/stores/auth'
import OperatorResources from '@/components/operator/OperatorResources.vue'
const props = defineProps<{ resource: 'accounts' | 'groups' }>()
const auth = useAuthStore()
const Accounts = defineAsyncComponent(() => import('./AccountsView.vue').then(module => module.default))
const Groups = defineAsyncComponent(() => import('./GroupsView.vue').then(module => module.default))
const adminView = computed(() => props.resource === 'accounts' ? Accounts : Groups)
</script>
