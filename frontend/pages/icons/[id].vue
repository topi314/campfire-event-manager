<script setup lang="ts">
const route = useRoute();
await navigateTo(`/packs/${route.params.id}`, { redirectCode: 301, replace: true });
</script>
