import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { api } from "@/lib/api";
import { Hash, Plus, RefreshCw, Search, Settings, Trash2 } from "lucide-react";
import React, { useEffect, useState } from "react";

interface Topic {
  id: number;
  name: string;
  messages: number;
  consumers: number;
  producers: number;
  partitions: number;
  replication: number;
  retention: string;
  status: "active" | "inactive" | "error";
  last_activity: string;
}

const Topics: React.FC = () => {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(true);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [newTopicName, setNewTopicName] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Settings modal state
  const [settingsModalOpen, setSettingsModalOpen] = useState(false);
  const [selectedTopic, setSelectedTopic] = useState<Topic | null>(null);
  const [topicSettings, setTopicSettings] = useState({
    partitions: 1,
    replication: 1,
    retention: "7 days",
  });

  const fetchTopics = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.get("/topics");
      console.log("Topics API response:", res.data);

      // Backend response format: {success: true, data: {topics: [...], count: X}, timestamp: ...}
      if (res.data?.data?.topics) {
        const formattedTopics = res.data.data.topics.map(
          (topic: any, index: number) => {
            // Backend now returns detailed topic objects instead of just names
            if (typeof topic === "string") {
              // Fallback for old format (just names)
              return {
                id: index + 1,
                name: topic,
                partitions: 1,
                replication: 1,
                messages: 0,
                consumers: 0,
                producers: 0,
                status: "active" as const,
                retention: "7 days",
                last_activity: new Date().toISOString(),
              };
            } else {
              // New format with detailed topic info
              return {
                id: index + 1,
                name: topic.name,
                partitions: topic.partitions || 1,
                replication: topic.replication || 1,
                messages: topic.messages || 0,
                consumers: topic.consumers || 0,
                producers: topic.producers || 0,
                status: topic.status || ("active" as const),
                retention: topic.retention || "7 days",
                last_activity: topic.created_at
                  ? new Date(topic.created_at * 1000).toISOString()
                  : new Date().toISOString(),
              };
            }
          }
        );
        setTopics(formattedTopics);
      } else {
        setTopics([]);
      }
    } catch (e) {
      setError("Konular alınamadı. Lütfen bağlantınızı kontrol edin.");
      setTopics([]);
    }
    setLoading(false);
  };

  // Topic oluştur
  const createTopic = async () => {
    if (!newTopicName.trim()) return;

    const topicName = newTopicName.trim();
    setCreating(true);
    setError(null);

    // Optimistic UI update
    const newTopic: Topic = {
      id: Date.now(),
      name: topicName,
      partitions: 1,
      replication: 1,
      messages: 0,
      consumers: 0,
      producers: 0,
      status: "active",
      retention: "7 days",
      last_activity: new Date().toISOString(),
    };

    setTopics((prev) => [...prev, newTopic]);
    setNewTopicName("");
    setCreateModalOpen(false);

    try {
      await api.post("/topics", { name: topicName });
      console.log("Topic created successfully");
      // Refresh to get accurate data
      await fetchTopics();
    } catch (error) {
      setError("Konu oluşturulamadı. Lütfen tekrar deneyin.");
      setTopics((prev) => prev.filter((t) => t.id !== newTopic.id));
    }
    setCreating(false);
  };

  // Topic sil
  const deleteTopic = async (topicName: string) => {
    if (!confirm(`Are you sure you want to delete topic "${topicName}"?`))
      return;

    // Optimistic UI update
    const originalTopics = topics;
    setTopics((prev) => prev.filter((t) => t.name !== topicName));

    try {
      await api.delete(`/topics/${topicName}`);
      console.log("Topic deleted successfully");
    } catch (error) {
      setError("Konu silinemedi. Lütfen tekrar deneyin.");
      setTopics(originalTopics);
    }
  };

  // Topic settings
  const openTopicSettings = (topic: Topic) => {
    setSelectedTopic(topic);
    setTopicSettings({
      partitions: topic.partitions,
      replication: topic.replication,
      retention: topic.retention,
    });
    setSettingsModalOpen(true);
  };

  const updateTopicSettings = async () => {
    if (!selectedTopic) return;

    try {
      await api.put(`/topics/${selectedTopic.name}`, {
        partitions: topicSettings.partitions,
        replication: topicSettings.replication,
      });
      setSettingsModalOpen(false);
      await fetchTopics(); // Listeyi yenile
      console.log("Topic settings updated successfully");
    } catch (error) {
      console.error("Failed to update topic settings:", error);
    }
  };

  const filteredTopics = topics.filter((topic) =>
    topic.name.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const getStatusColor = (status: Topic["status"]) => {
    switch (status) {
      case "active":
        return "bg-green-100 text-green-800";
      case "inactive":
        return "bg-yellow-100 text-yellow-800";
      case "error":
        return "bg-red-100 text-red-800";
      default:
        return "bg-gray-100 text-gray-800";
    }
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

  useEffect(() => {
    fetchTopics();

    // Auto-refresh every 10 seconds
    const interval = setInterval(fetchTopics, 10000);

    return () => clearInterval(interval);
  }, []);

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
          <h1 className="text-3xl font-bold">Topics</h1>
          <p className="text-muted-foreground">
            Manage message topics and their configurations
          </p>
        </div>
        <div className="flex space-x-2">
          <Button variant="outline" onClick={fetchTopics} disabled={loading}>
            <RefreshCw
              className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`}
            />
            Refresh
          </Button>
          <Button onClick={() => setCreateModalOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Create Topic
          </Button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Topics</CardTitle>
            <Hash className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{topics.length}</div>
            <p className="text-xs text-muted-foreground">Configured topics</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Topics</CardTitle>
            <div className="h-2 w-2 bg-green-500 rounded-full" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {topics.filter((t) => t.status === "active").length}
            </div>
            <p className="text-xs text-muted-foreground">Currently active</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Total Messages
            </CardTitle>
            <Hash className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatNumber(topics.reduce((sum, t) => sum + t.messages, 0))}
            </div>
            <p className="text-xs text-muted-foreground">Across all topics</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Total Partitions
            </CardTitle>
            <Hash className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {topics.reduce((sum, t) => sum + t.partitions, 0)}
            </div>
            <p className="text-xs text-muted-foreground">Total partitions</p>
          </CardContent>
        </Card>
      </div>

      {/* Topics Table */}
      <Card>
        <CardHeader>
          <CardTitle>Topics Overview</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center space-x-2 mb-4">
            <div className="relative flex-1">
              <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search topics..."
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
                  <TableHead>Topic Name</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Messages</TableHead>
                  <TableHead>Consumers</TableHead>
                  <TableHead>Producers</TableHead>
                  <TableHead>Partitions</TableHead>
                  <TableHead>Retention</TableHead>
                  <TableHead>Last Message</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredTopics.map((topic) => (
                  <TableRow key={topic.name}>
                    <TableCell className="font-medium">
                      <div className="flex items-center space-x-2">
                        <Hash className="h-4 w-4 text-muted-foreground" />
                        <span>{topic.name}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge className={getStatusColor(topic.status)}>
                        {topic.status}
                      </Badge>
                    </TableCell>
                    <TableCell>{formatNumber(topic.messages)}</TableCell>
                    <TableCell>{topic.consumers}</TableCell>
                    <TableCell>{topic.producers}</TableCell>
                    <TableCell>{topic.partitions}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{topic.retention}</Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {getRelativeTime(new Date(topic.last_activity))}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center space-x-2">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => openTopicSettings(topic)}
                        >
                          <Settings className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => deleteTopic(topic.name)}
                        >
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

      {/* Create Topic Modal */}
      {createModalOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-96 max-w-md">
            <h2 className="text-xl font-semibold mb-4">Create New Topic</h2>
            <div className="space-y-4">
              <div>
                <Label htmlFor="topicName">Topic Name</Label>
                <Input
                  id="topicName"
                  placeholder="Enter topic name..."
                  value={newTopicName}
                  onChange={(e) => setNewTopicName(e.target.value)}
                  disabled={creating}
                />
              </div>
              <div className="flex justify-end space-x-2">
                <Button
                  variant="outline"
                  onClick={() => setCreateModalOpen(false)}
                  disabled={creating}
                >
                  Cancel
                </Button>
                <Button onClick={createTopic} disabled={creating}>
                  {creating ? "Creating..." : "Create Topic"}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Settings Topic Modal */}
      {settingsModalOpen && selectedTopic && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-96 max-w-md">
            <h2 className="text-xl font-semibold mb-4">Topic Settings</h2>
            <div className="space-y-4">
              <div>
                <Label htmlFor="partitions">Partitions</Label>
                <Input
                  id="partitions"
                  type="number"
                  value={topicSettings.partitions}
                  onChange={(e) =>
                    setTopicSettings({
                      ...topicSettings,
                      partitions: Number(e.target.value),
                    })
                  }
                />
              </div>
              <div>
                <Label htmlFor="replication">Replication</Label>
                <Input
                  id="replication"
                  type="number"
                  value={topicSettings.replication}
                  onChange={(e) =>
                    setTopicSettings({
                      ...topicSettings,
                      replication: Number(e.target.value),
                    })
                  }
                />
              </div>
              <div>
                <Label htmlFor="retention">Retention</Label>
                <Input
                  id="retention"
                  placeholder="e.g., 7 days"
                  value={topicSettings.retention}
                  onChange={(e) =>
                    setTopicSettings({
                      ...topicSettings,
                      retention: e.target.value,
                    })
                  }
                />
              </div>
              <div className="flex justify-end space-x-2">
                <Button
                  variant="outline"
                  onClick={() => setSettingsModalOpen(false)}
                >
                  Cancel
                </Button>
                <Button onClick={updateTopicSettings}>Save Settings</Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Topics;
