import type { ClientOptions, HealthStatus, APIResponse } from './types';
import { Producer } from './producer';
import { Consumer } from './consumer';
import { ConsumerGroupClient } from './consumer-group';
import { TransactionClient } from './transaction';

export class PortaskClient {
  private baseURL: string;
  private apiKey?: string;
  private timeout: number;
  private headers: Record<string, string>;

  private _producer?: Producer;
  private _consumer?: Consumer;
  private _consumerGroup?: ConsumerGroupClient;
  private _transaction?: TransactionClient;

  constructor(options: ClientOptions) {
    this.baseURL = options.baseURL.replace(/\/$/, ''); // Remove trailing slash
    this.apiKey = options.apiKey;
    this.timeout = options.timeout || 30000;
    this.headers = options.headers || {};
  }

  // ==================== Component Accessors ====================

  producer(): Producer {
    if (!this._producer) {
      this._producer = new Producer(this);
    }
    return this._producer;
  }

  consumer(): Consumer {
    if (!this._consumer) {
      this._consumer = new Consumer(this);
    }
    return this._consumer;
  }

  consumerGroup(): ConsumerGroupClient {
    if (!this._consumerGroup) {
      this._consumerGroup = new ConsumerGroupClient(this);
    }
    return this._consumerGroup;
  }

  transaction(): TransactionClient {
    if (!this._transaction) {
      this._transaction = new TransactionClient(this);
    }
    return this._transaction;
  }

  // ==================== Health Check ====================

  async health(): Promise<HealthStatus> {
    const response = await this.get<HealthStatus>('/health');
    return response;
  }

  // ==================== HTTP Methods ====================

  async get<T = any>(path: string): Promise<T> {
    return this.request<T>('GET', path);
  }

  async post<T = any>(path: string, body?: any): Promise<T> {
    return this.request<T>('POST', path, body);
  }

  async put<T = any>(path: string, body?: any): Promise<T> {
    return this.request<T>('PUT', path, body);
  }

  async delete<T = any>(path: string): Promise<T> {
    return this.request<T>('DELETE', path);
  }

  private async request<T>(
    method: string,
    path: string,
    body?: any
  ): Promise<T> {
    const url = `${this.baseURL}${path}`;

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
      ...this.headers,
    };

    if (this.apiKey) {
      headers['Authorization'] = `Bearer ${this.apiKey}`;
    }

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(url, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`HTTP ${response.status}: ${errorText}`);
      }

      // Handle empty responses
      const contentType = response.headers.get('content-type');
      if (!contentType || !contentType.includes('application/json')) {
        return {} as T;
      }

      const data = await response.json();

      // Check for API error response
      if (data.success === false && data.error) {
        throw new Error(data.error);
      }

      return data as T;
    } catch (error: any) {
      if (error.name === 'AbortError') {
        throw new Error(`Request timeout after ${this.timeout}ms`);
      }
      throw error;
    }
  }
}

// Export factory function for convenience
export function createClient(options: ClientOptions): PortaskClient {
  return new PortaskClient(options);
}

