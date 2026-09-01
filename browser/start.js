try {
  await import('./player.js');
  await import('./runtime.js');
} catch (error) {
  console.error('[White Raven startup]', error);
  const status = document.getElementById('browser-status');
  status.dataset.state = 'error';
  status.textContent = 'Could not load the player. Check your connection and reload.';
}
