<template>
	<nav class="navbar">
		<div class="container is-fluid">
			<div class="navbar-brand">
				<span class="navbar-item brand-text has-text-weight-bold">
					gitdeploy
				</span>
			</div>
			<div class="navbar-menu">
				<div class="navbar-start">
					<router-link class="navbar-item" to="/">Dashboard</router-link>
					<router-link class="navbar-item" to="/sites">Sites</router-link>
					<router-link class="navbar-item" to="/jobs">Jobs</router-link>
				</div>
			</div>
		</div>
	</nav>
</template>

<script>
module.exports = {
	name: 'navbar'
};
</script>
