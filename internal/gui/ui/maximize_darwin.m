//go:build darwin

#import <Cocoa/Cocoa.h>
#include <stdint.h>

void maximizeNSWindow(uintptr_t handle) {
	NSWindow *window = (NSWindow *)handle;
	NSScreen *screen = [window screen];
	if (screen == nil) {
		screen = [NSScreen mainScreen];
	}
	if (screen == nil) {
		return;
	}
	[window setFrame:[screen visibleFrame] display:YES];
}
