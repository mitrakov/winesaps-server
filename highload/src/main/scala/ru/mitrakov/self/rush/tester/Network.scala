package ru.mitrakov.self.rush.tester

import java.net.{DatagramPacket, DatagramSocket, InetAddress}

import akka.actor.ActorRef

/**
  * Created by mitrakov on 04.07.2017.
  */
class Network(host: String, port: Int) {
  private var router: ActorRef = _
  private val socket = new DatagramSocket()
  private val addr = InetAddress getByName host
  private val thread = new Thread(new Runnable {
    override def run(): Unit = {
      while (true) {
        val packet = new DatagramPacket(new Array(1024), 1024)
        socket receive packet
        router ! packet
      }
    }
  })

  thread setDaemon true

  def setRouter(router: ActorRef): Unit = {
    this.router = router
    thread.start()
  }

  def send(lst: List[Int]): Unit = {
    socket send new DatagramPacket(lst.map{_.toByte}.toArray, lst.length, addr, port)
  }
}
