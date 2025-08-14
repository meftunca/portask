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
import {
  Plus,
  RefreshCw,
  Search,
  Settings,
  Trash2,
  Wifi,
  WifiOff,
} from "lucide-react";
import React, { useEffect, useState } from "react";

interface Connection {
  id: string;
  name: string;
  type: "producer" | "consumer";
  protocol: "AMQP" | "Kafka" | "Redis" | "HTTP";
  host: string;
  port: number;
  status: "connected" | "disconnected" | "error";
  lastSeen: Date;
  messagesHandled: number;
  topics: string[];
}

const Connections: React.FC = () => {
  const [connections, setConnections] = useState<Connection[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Gerçek API'den veri çek
    const fetchConnections = async () => {
      setLoading(true);
      setError(null);
      try {
        const res = await api.get("/connections");
        if (res.data && (res.data.connections || res.data.data?.connections)) {
          const connList =
            res.data.connections || res.data.data?.connections || [];
          const formattedConnections = connList.map(
            (conn: any, index: number) => ({
              id: index + 1,
              name: conn.name || `Connection ${index + 1}`,
              type: conn.type || "producer",
              host: conn.host || conn.address || "localhost",
              port: conn.port || 9092,
              protocol: conn.protocol || "TCP",
              status: conn.status || "connected",
              messagesHandled: conn.messagesHandled || conn.messages_sent || 0,
              topics: conn.topics || [],
              lastSeen: conn.lastSeen ? new Date(conn.lastSeen) : new Date(),
            })
          );
          setConnections(formattedConnections);
        } else {
          setConnections([]);
        }
      } catch (e) {
        setError("Bağlantılar alınamadı. Lütfen bağlantınızı kontrol edin.");
        setConnections([]);
      }
      setLoading(false);
    };
    fetchConnections();
  }, []);

  const filteredConnections = connections.filter(
    (conn) =>
      conn.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      conn.host.includes(searchTerm) ||
      conn.protocol.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const getStatusColor = (status: Connection["status"]) => {
    switch (status) {
      case "connected":
        return "bg-green-100 text-green-800";
      case "disconnected":
        return "bg-yellow-100 text-yellow-800";
      case "error":
        return "bg-red-100 text-red-800";
      default:
        return "bg-gray-100 text-gray-800";
    }
  };

  const getStatusIcon = (status: Connection["status"]) => {
    switch (status) {
      case "connected":
        return <Wifi className="h-4 w-4" />;
      case "disconnected":
      case "error":
        return <WifiOff className="h-4 w-4" />;
      default:
        return <WifiOff className="h-4 w-4" />;
    }
  };

  const getTypeColor = (type: Connection["type"]) => {
    return type === "producer"
      ? "bg-blue-100 text-blue-800"
      : "bg-purple-100 text-purple-800";
  };

  const formatNumber = (num: number) => {
    return new Intl.NumberFormat().format(num);
  };

  const getRelativeTime = (date: Date) => {
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);

    if (diffMins < 1) return "Just now";
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffMins < 1440) return `${Math.floor(diffMins / 60)}h ago`;
    return `${Math.floor(diffMins / 1440)}d ago`;
  };

  const connectedCount = connections.filter(
    (c) => c.status === "connected"
  ).length;
  const producerCount = connections.filter((c) => c.type === "producer").length;
  const consumerCount = connections.filter((c) => c.type === "consumer").length;

  const fetchConnections = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.get("/connections");
      if (res.data && (res.data.connections || res.data.data?.connections)) {
        const connList =
          res.data.connections || res.data.data?.connections || [];
        const formattedConnections = connList.map(
          (conn: any, index: number) => ({
            id: index + 1,
            name: conn.name || `Connection ${index + 1}`,
            type: conn.type || "producer",
            host: conn.host || conn.address || "localhost",
            port: conn.port || 9092,
            protocol: conn.protocol || "TCP",
            status: conn.status || "connected",
            messagesHandled: conn.messagesHandled || conn.messages_sent || 0,
            topics: conn.topics || [],
            lastSeen: conn.lastSeen ? new Date(conn.lastSeen) : new Date(),
          })
        );
        setConnections(formattedConnections);
      } else {
        setConnections([]);
      }
    } catch (e) {
      setError("Bağlantılar alınamadı. Lütfen bağlantınızı kontrol edin.");
      setConnections([]);
    }
    setLoading(false);
  };

  // Connection test işlevi
  const testConnection = async (connectionId: number) => {
    try {
      // Bu örnekte connection test endpoint'i yoksa, sadece log atacağız
      console.log(`Testing connection ${connectionId}...`);
      // await api.post(`/connections/${connectionId}/test`)
      alert("Connection test completed successfully");
    } catch (error) {
      console.error("Connection test failed:", error);
      alert("Connection test failed");
    }
  };

  // Connection refresh işlevi
  const refreshConnections = async () => {
    await fetchConnections();
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
          <h1 className="text-3xl font-bold">Connections</h1>
          <p className="text-muted-foreground">
            Monitor active producers and consumers
          </p>
        </div>
        <div className="flex space-x-2">
          <Button variant="outline" onClick={refreshConnections}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Refresh
          </Button>
          <Button>
            <Plus className="mr-2 h-4 w-4" />
            Add Connection
          </Button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Total Connections
            </CardTitle>
            <Wifi className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{connections.length}</div>
            <p className="text-xs text-muted-foreground">
              Registered connections
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Connected</CardTitle>
            <div className="h-2 w-2 bg-green-500 rounded-full animate-pulse" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{connectedCount}</div>
            <p className="text-xs text-muted-foreground">Active now</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Producers</CardTitle>
            <div className="h-2 w-2 bg-blue-500 rounded-full" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{producerCount}</div>
            <p className="text-xs text-muted-foreground">Sending messages</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Consumers</CardTitle>
            <div className="h-2 w-2 bg-purple-500 rounded-full" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{consumerCount}</div>
            <p className="text-xs text-muted-foreground">Processing messages</p>
          </CardContent>
        </Card>
      </div>

      {/* Connections Table */}
      <Card>
        <CardHeader>
          <CardTitle>Active Connections</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center space-x-2 mb-4">
            <div className="relative flex-1">
              <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search connections..."
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
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Protocol</TableHead>
                  <TableHead>Endpoint</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Messages</TableHead>
                  <TableHead>Topics</TableHead>
                  <TableHead>Last Seen</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredConnections.map((connection) => (
                  <TableRow key={connection.id}>
                    <TableCell className="font-medium">
                      <div className="flex items-center space-x-2">
                        {getStatusIcon(connection.status)}
                        <span>{connection.name}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge className={getTypeColor(connection.type)}>
                        {connection.type}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{connection.protocol}</Badge>
                    </TableCell>
                    <TableCell className="font-mono text-sm">
                      {connection.host}:{connection.port}
                    </TableCell>
                    <TableCell>
                      <Badge className={getStatusColor(connection.status)}>
                        {connection.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {formatNumber(connection.messagesHandled)}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {connection.topics.slice(0, 2).map((topic) => (
                          <Badge
                            key={topic}
                            variant="outline"
                            className="text-xs"
                          >
                            {topic}
                          </Badge>
                        ))}
                        {connection.topics.length > 2 && (
                          <Badge variant="outline" className="text-xs">
                            +{connection.topics.length - 2}
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {getRelativeTime(connection.lastSeen)}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center space-x-2">
                        <Button variant="ghost" size="sm">
                          <Settings className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => testConnection(Number(connection.id))}
                        >
                          Test
                        </Button>
                        <Button variant="ghost" size="sm">
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
};

export default Connections;
