import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { api } from "@/lib/api";
import MonacoEditor from "@monaco-editor/react";
import {
  Clock,
  Eye,
  MessageSquare,
  RefreshCw,
  Search,
  Send,
} from "lucide-react";
import React, { useEffect, useState } from "react";

interface Message {
  id: string;
  topic: string;
  partition: number;
  offset: number;
  key?: string;
  value: any;
  headers?: Record<string, string>;
  timestamp: string;
  size: number;
  ttl?: number;
}

const Messages: React.FC = () => {
  const [messages, setMessages] = useState<Message[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(true);
  const [publishModalOpen, setPublishModalOpen] = useState(false);
  const [newMessage, setNewMessage] = useState({ topic: "", payload: "" });
  const [publishing, setPublishing] = useState(false);
  const [selectedTopic, setSelectedTopic] = useState("");
  const [error, setError] = useState<string | null>(null);

  const fetchMessages = async (topicOverride?: string) => {
    setLoading(true);
    setError(null);
    try {
      const topicToFetch =
        topicOverride || selectedTopic || newMessage.topic || "";
      if (!topicToFetch) {
        setMessages([]);
        setLoading(false);
        return;
      }
      const res = await api.post("/messages/fetch", {
        topic: topicToFetch,
        partition: 0,
        offset: 0,
        limit: 100,
      });

      console.log("Messages API response:", res.data);

      // Backend response format: {success: true, data: {messages: [...]}, timestamp: ...}
      if (res.data?.data?.messages) {
        const formattedMessages = res.data.data.messages.map(
          (msg: any, index: number) => ({
            id: msg.id || `msg_${index}`,
            topic: msg.topic || "unknown",
            partition: msg.partition || 0,
            offset: msg.offset || 0,
            key: msg.key,
            value: msg.value,
            headers: msg.headers || {},
            timestamp: msg.timestamp
              ? new Date(msg.timestamp).toISOString()
              : new Date().toISOString(),
            size: JSON.stringify(msg.value || "").length,
            ttl: msg.ttl,
          })
        );
        setMessages(formattedMessages);
      } else {
        setMessages([]);
      }
    } catch (e) {
      setError("Mesajlar alınamadı. Lütfen bağlantınızı kontrol edin.");
      setMessages([]);
    }
    setLoading(false);
  };

  // Message publish et
  const publishMessage = async () => {
    if (!newMessage.topic.trim() || !newMessage.payload.trim()) return;

    setPublishing(true);
    setError(null);
    try {
      await api.post("/messages/publish", {
        topic: newMessage.topic.trim(),
        value: newMessage.payload.trim(),
      });
      setNewMessage({ topic: "", payload: "" });
      setPublishModalOpen(false);
      setSelectedTopic(newMessage.topic.trim());
      await fetchMessages(newMessage.topic.trim()); // Listeyi yenile
    } catch (error) {
      setError("Mesaj gönderilemedi. Lütfen tekrar deneyin.");
    }
    setPublishing(false);
  };

  useEffect(() => {
    fetchMessages();

    // Auto-refresh every 10 seconds
    const interval = setInterval(fetchMessages, 10000);

    return () => clearInterval(interval);
  }, [selectedTopic]);

  const filteredMessages = messages.filter(
    (msg) =>
      msg.topic.toLowerCase().includes(searchTerm.toLowerCase()) ||
      JSON.stringify(msg.value).toLowerCase().includes(searchTerm.toLowerCase())
  );

  const formatValue = (value: any) => {
    if (typeof value === "string") return value;
    return JSON.stringify(value);
  };

  const formatTimestamp = (timestamp: string) => {
    return new Date(timestamp).toLocaleString();
  };

  return (
    <div className="space-y-6">
      {error && (
        <div className="bg-red-100 text-red-700 px-4 py-2 rounded mb-2">
          {error}
        </div>
      )}
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Messages</h1>
          <p className="text-muted-foreground">
            Monitor and manage message queue activity
          </p>
        </div>
        <div className="flex space-x-2">
          <Button
            variant="outline"
            onClick={() => fetchMessages()}
            disabled={loading}
          >
            <RefreshCw
              className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`}
            />
            Refresh
          </Button>
          <Button onClick={() => setPublishModalOpen(true)}>
            <Send className="mr-2 h-4 w-4" />
            Send Message
          </Button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {/* Total Messages Card */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Total Messages
            </CardTitle>
            <MessageSquare className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{messages.length}</div>
            <p className="text-xs text-muted-foreground">Current session</p>
          </CardContent>
        </Card>
        {/* Unique Topics Card */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Unique Topics</CardTitle>
            <Clock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {new Set(messages.map((m) => m.topic)).size}
            </div>
            <p className="text-xs text-muted-foreground">Active topics</p>
          </CardContent>
        </Card>
        {/* Total Size Card */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Size</CardTitle>
            <div className="h-2 w-2 bg-blue-500 rounded-full" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {messages.reduce((acc, m) => acc + m.size, 0)}B
            </div>
            <p className="text-xs text-muted-foreground">Data volume</p>
          </CardContent>
        </Card>
        {/* Latest Card */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Latest</CardTitle>
            <div className="h-2 w-2 bg-green-500 rounded-full animate-pulse" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {messages.length > 0 ? "Now" : "None"}
            </div>
            <p className="text-xs text-muted-foreground">Last message</p>
          </CardContent>
        </Card>
      </div>

      {/* Search and Messages Table */}
      <Card>
        <CardHeader>
          <CardTitle>Message Queue</CardTitle>
          <div className="flex items-center space-x-2 mt-2">
            <Input
              placeholder="Topic ile filtrele..."
              value={selectedTopic}
              onChange={(e) => setSelectedTopic(e.target.value)}
              className="max-w-xs"
            />
          </div>
        </CardHeader>
        <CardContent>
          <div className="flex items-center space-x-2 mb-4">
            <div className="relative flex-1">
              <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search messages..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="pl-8"
              />
            </div>
          </div>

          {loading ? (
            <div className="flex items-center justify-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>Topic</TableHead>
                  <TableHead>Partition</TableHead>
                  <TableHead>Offset</TableHead>
                  <TableHead>Value</TableHead>
                  <TableHead>Size</TableHead>
                  <TableHead>Timestamp</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredMessages.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className="text-center py-8 text-muted-foreground"
                    >
                      No messages found
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredMessages.map((message) => (
                    <TableRow key={message.id}>
                      <TableCell className="font-mono text-sm">
                        {message.id.substring(0, 8)}...
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">{message.topic}</Badge>
                      </TableCell>
                      <TableCell>{message.partition}</TableCell>
                      <TableCell>{message.offset}</TableCell>
                      <TableCell className="max-w-xs truncate">
                        <code className="text-xs bg-muted px-1 py-0.5 rounded">
                          {formatValue(message.value)}
                        </code>
                      </TableCell>
                      <TableCell>{message.size}B</TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {formatTimestamp(message.timestamp)}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          <Button variant="ghost" size="sm">
                            <Eye className="h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Publish Message Modal */}
      {publishModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="w-full max-w-md rounded-lg bg-white p-6">
            <h2 className="text-lg font-semibold mb-4">Send Message</h2>
            <div className="mb-4">
              <label className="block text-sm font-medium mb-1">Topic</label>
              <Input
                placeholder="Enter topic name"
                value={newMessage.topic}
                onChange={(e) =>
                  setNewMessage({ ...newMessage, topic: e.target.value })
                }
              />
            </div>
            <div className="mb-4">
              <label className="block text-sm font-medium mb-1">Message</label>
              <div style={{ height: 180 }}>
                <MonacoEditor
                  height="180px"
                  language="json"
                  theme="vs-light"
                  value={newMessage.payload}
                  options={{ minimap: { enabled: false }, fontSize: 14 }}
                  onChange={(val: string | undefined) =>
                    setNewMessage({ ...newMessage, payload: val || "" })
                  }
                />
              </div>
            </div>
            <div className="flex justify-end space-x-2">
              <Button
                variant="outline"
                onClick={() => setPublishModalOpen(false)}
                className="flex-1"
              >
                Cancel
              </Button>
              <Button
                onClick={publishMessage}
                disabled={publishing}
                className="flex-1"
              >
                {publishing ? "Sending..." : "Send Message"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Messages;
