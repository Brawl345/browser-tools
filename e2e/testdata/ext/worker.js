chrome.runtime.onStartup.addListener(() => {});

chrome.action.onClicked.addListener(() => {
  chrome.tabs.create({ url: chrome.runtime.getURL("opened.html"), active: false });
});

// Like Tampermonkey, the .user.js listener only exists once an async setup has finished.
chrome.storage.local.get(null).then(() => {
  chrome.webNavigation.onBeforeNavigate.addListener(({ tabId, frameId, url }) => {
    if (frameId === 0 && new URL(url).pathname.endsWith(".user.js")) {
      chrome.tabs.update(tabId, { url: chrome.runtime.getURL("install.html") + "?src=" + encodeURIComponent(url) });
    }
  });
});
