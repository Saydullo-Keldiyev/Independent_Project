type MessageHandler = (data: any) => void;

class WebSocketService {
  private ws: WebSocket | null = null;
  private handlers: Map<string, MessageHandler[]> = new Map();
  private reconnectAttempts = 0;
  private maxReconnects = 5;
  private url = '';

  connect(auctionId: string) {
    const baseUrl = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8082';
    const token = localStorage.getItem('access_token');
    this.url = `${baseUrl}/api/v1/ws/${auctionId}?token=${token}`;

    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      console.log('WebSocket connected:', auctionId);
      this.reconnectAttempts = 0;
    };

    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        const type = data.type || 'message';
        const handlers = this.handlers.get(type) || [];
        handlers.forEach(h => h(data));
        // Also fire 'all' handlers
        (this.handlers.get('*') || []).forEach(h => h(data));
      } catch (e) {
        console.error('WS parse error:', e);
      }
    };

    this.ws.onclose = () => {
      if (this.reconnectAttempts < this.maxReconnects) {
        this.reconnectAttempts++;
        setTimeout(() => this.connect(auctionId), 2000 * this.reconnectAttempts);
      }
    };

    this.ws.onerror = (err) => console.error('WS error:', err);
  }

  on(event: string, handler: MessageHandler) {
    if (!this.handlers.has(event)) this.handlers.set(event, []);
    this.handlers.get(event)!.push(handler);
  }

  off(event: string, handler: MessageHandler) {
    const handlers = this.handlers.get(event);
    if (handlers) {
      this.handlers.set(event, handlers.filter(h => h !== handler));
    }
  }

  disconnect() {
    this.ws?.close();
    this.ws = null;
    this.handlers.clear();
    this.reconnectAttempts = this.maxReconnects; // prevent reconnect
  }
}

export const wsService = new WebSocketService();
