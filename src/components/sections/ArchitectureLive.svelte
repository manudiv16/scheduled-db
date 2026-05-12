<script lang="ts">
  import SectionHeader from '../ui/SectionHeader.svelte';
  import ClusterTopology from '../viz/ClusterTopology.svelte';
</script>

<section id="architecture" class="container architecture-section">
  <SectionHeader 
    code="SYS-02" 
    title="ARCHITECTURE TOPOLOGY" 
    subtitle="Live view of the Raft cluster and related services"
  />
  
  <div class="topology-wrapper">
    <ClusterTopology />
  </div>
</section>

<style>
  .architecture-section {
    padding: var(--spacing-2xl) 0;
  }
  .topology-wrapper {
    width: 100%;
    max-width: 1000px;
    margin: 0 auto;
  }
</style>
