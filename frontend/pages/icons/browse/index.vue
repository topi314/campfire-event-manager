<script setup lang="ts">
const route = useRoute();
await navigateTo({ path: "/packs/browse", query: route.query }, { redirectCode: 301, replace: true });
</script>
