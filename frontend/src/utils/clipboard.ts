export const copyToClipboard = async (text: string): Promise<boolean> => {
  // 优先使用现代 Clipboard API
  if (navigator.clipboard && navigator.clipboard.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch (err) {
      console.warn('Clipboard API 失败，尝试回退方案...', err);
    }
  }

  // 回退方案：使用 document.execCommand (支持 HTTP 环境)
  try {
    const textArea = document.createElement('textarea');
    textArea.value = text;

    // 避免页面滚动
    textArea.style.position = 'fixed';
    textArea.style.left = '-9999px';
    textArea.style.top = '0';

    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    const successful = document.execCommand('copy');
    document.body.removeChild(textArea);

    return successful;
  } catch (err) {
    console.error('复制失败:', err);
    return false;
  }
};
