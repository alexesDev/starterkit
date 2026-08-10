namespace $.$$ {
	export class $starterkit_catalog extends $.$starterkit_catalog {
		override menu_tools() {
			return [
				...this.actions(),
				this.close_icon(),
			]
		}
	}
}
