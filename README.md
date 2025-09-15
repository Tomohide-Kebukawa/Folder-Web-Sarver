# Folder-Web-Sarver [▶Japanese](./README_JP.md)
Folder Web Server allows you to view the contents of multiple folders in your browser.

## Discription

+ Allows you to view the contents of a specified folder in a browser.
+ For image files, a dedicated HTML page allows easy browsing.
  + Image browsing uses horizontal scrolling.
  + Images on the same level can be viewed by scrolling horizontally, giving the feeling of turning a page.
  + If a file named `__option_R2L__` is present in the folder, the horizontal scrolling direction will be reversed.
+ Server settings are configured using the settings.json file located on the same level.

## setting.json

```json
{
	"config": {
		"server": { "port": 9999 },
		"templates": {
			"index": "./Templates/index.html",
			"folder": "./Templates/folder.html",
			"image": "./Templates/image.html",
			"imageR2L": "./Templates/imageR2L.html"
		}
	},
	"folders": [
		"/VolumeA/Folder-1/",
		"/VolumeB/Folder-2/",
		"/VolumeB/Folder-3/"
	],
	"ignores": [
		"\\..*",
		"Icon"
	]
}
```
